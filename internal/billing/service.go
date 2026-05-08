package billing

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	stripe "github.com/stripe/stripe-go/v84"
	"github.com/stripe/stripe-go/v84/webhook"

	"github.com/phantompunk/confide/internal/db/queries"
	"github.com/phantompunk/confide/internal/permission"
)

var (
	ErrForbidden      = errors.New("owner role required")
	ErrNotFound       = errors.New("workspace not found")
	ErrStripeDisabled = errors.New("stripe not configured")
	ErrNoCustomer     = errors.New("no stripe customer — workspace has never subscribed")
	ErrInvalidPlan    = errors.New("invalid plan")
)

// DB is the subset of queries used by billing.Service.
type DB interface {
	GetWorkspaceForBilling(ctx context.Context, id string) (queries.GetWorkspaceForBillingRow, error)
	CountWorkspaceMembers(ctx context.Context, workspaceID string) (int64, error)
	CountFormsByWorkspace(ctx context.Context, workspaceID string) (int64, error)
	CountMonthlyResponses(ctx context.Context, workspaceID string) (int64, error)
	SetStripeCustomerID(ctx context.Context, arg queries.SetStripeCustomerIDParams) error
	SetStripeSubscriptionID(ctx context.Context, arg queries.SetStripeSubscriptionIDParams) error
	UpdateWorkspacePlan(ctx context.Context, arg queries.UpdateWorkspacePlanParams) error
	GetWorkspaceByStripeCustomerID(ctx context.Context, stripeCustomerID pgtype.Text) (queries.GetWorkspaceByStripeCustomerIDRow, error)
}

// Service handles workspace billing operations.
type Service struct {
	log                 zerolog.Logger
	db                  DB
	stripeSecretKey     string
	stripeWebhookSecret string
	stripePriceIDPro    string
	stripePriceIDOrg    string
}

func NewService(pool *pgxpool.Pool, stripeSecretKey, stripeWebhookSecret, stripePriceIDPro, stripePriceIDOrg string) *Service {
	return &Service{
		log:                 log.With().Str("module", "billing").Logger(),
		db:                  queries.New(pool),
		stripeSecretKey:     stripeSecretKey,
		stripeWebhookSecret: stripeWebhookSecret,
		stripePriceIDPro:    stripePriceIDPro,
		stripePriceIDOrg:    stripePriceIDOrg,
	}
}

// BillingInfo contains the billing summary returned to the owner.
type BillingInfo struct {
	Plan                 string
	PlanStatus           string
	PlanPeriodEnd        *time.Time
	MemberCount          int64
	FormCount            int64
	MonthlyResponseCount int64
	HasStripeCustomer    bool
}

// GetInfo returns the billing summary for a workspace. Owner only.
func (s *Service) GetInfo(ctx context.Context, workspaceID, callerRole string) (BillingInfo, error) {
	if !permission.Can(callerRole, permission.ActionManageBilling) {
		return BillingInfo{}, ErrForbidden
	}
	ws, err := s.db.GetWorkspaceForBilling(ctx, workspaceID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return BillingInfo{}, ErrNotFound
		}
		return BillingInfo{}, err
	}

	memberCount, err := s.db.CountWorkspaceMembers(ctx, workspaceID)
	if err != nil {
		return BillingInfo{}, err
	}
	formCount, err := s.db.CountFormsByWorkspace(ctx, workspaceID)
	if err != nil {
		return BillingInfo{}, err
	}
	monthlyCount, err := s.db.CountMonthlyResponses(ctx, workspaceID)
	if err != nil {
		return BillingInfo{}, err
	}

	info := BillingInfo{
		Plan:                 ws.Plan,
		PlanStatus:           ws.PlanStatus,
		MemberCount:          memberCount,
		FormCount:            formCount,
		MonthlyResponseCount: monthlyCount,
		HasStripeCustomer:    ws.StripeCustomerID.Valid,
	}
	if ws.PlanPeriodEnd.Valid {
		t := ws.PlanPeriodEnd.Time
		info.PlanPeriodEnd = &t
	}
	return info, nil
}

// Subscribe creates or upgrades a workspace subscription via Stripe Checkout.
// plan must be "pro" or "org". Lazily creates a Stripe Customer on first upgrade
// (workspace_id metadata only, no PII). Returns the Stripe Checkout Session URL. Owner only.
func (s *Service) Subscribe(ctx context.Context, workspaceID, callerRole, plan, successURL, cancelURL string) (string, error) {
	if s.stripeSecretKey == "" {
		return "", ErrStripeDisabled
	}
	priceID, err := s.priceIDForPlan(plan)
	if err != nil {
		return "", err
	}
	if !permission.Can(callerRole, permission.ActionManageBilling) {
		return "", ErrForbidden
	}
	ws, err := s.db.GetWorkspaceForBilling(ctx, workspaceID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ErrNotFound
		}
		return "", err
	}

	sc := stripe.NewClient(s.stripeSecretKey)

	customerID := ""
	if ws.StripeCustomerID.Valid {
		customerID = ws.StripeCustomerID.String
	} else {
		// Lazy-create — workspace_id metadata only, no PII.
		c, err := sc.V1Customers.Create(ctx, &stripe.CustomerCreateParams{
			Metadata: map[string]string{"workspace_id": workspaceID},
		})
		if err != nil {
			return "", err
		}
		s.log.Info().Str("workspace_id", workspaceID).Msg("created new stripe customer")
		if err := s.db.SetStripeCustomerID(ctx, queries.SetStripeCustomerIDParams{
			ID:               workspaceID,
			StripeCustomerID: pgtype.Text{String: c.ID, Valid: true},
		}); err != nil {
			return "", err
		}
		customerID = c.ID
	}

	sess, err := sc.V1CheckoutSessions.Create(ctx, &stripe.CheckoutSessionCreateParams{
		Customer: stripe.String(customerID),
		Mode:     stripe.String(string(stripe.CheckoutSessionModeSubscription)),
		AllowPromotionCodes: stripe.Bool(true),
		LineItems: []*stripe.CheckoutSessionCreateLineItemParams{
			{
				Price:    stripe.String(priceID),
				Quantity: stripe.Int64(1),
			},
		},
		SuccessURL: stripe.String(successURL),
		CancelURL:  stripe.String(cancelURL),
	})
	if err != nil {
		return "", err
	}
	return sess.URL, nil
}

// Portal creates a Stripe Billing Portal session for self-service plan management.
// Requires a Stripe customer to already exist. Owner only.
func (s *Service) Portal(ctx context.Context, workspaceID, callerRole, returnURL string) (string, error) {
	if s.stripeSecretKey == "" {
		return "", ErrStripeDisabled
	}
	if !permission.Can(callerRole, permission.ActionManageBilling) {
		return "", ErrForbidden
	}
	ws, err := s.db.GetWorkspaceForBilling(ctx, workspaceID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ErrNotFound
		}
		return "", err
	}
	if !ws.StripeCustomerID.Valid {
		return "", ErrNoCustomer
	}

	sc := stripe.NewClient(s.stripeSecretKey)
	sess, err := sc.V1BillingPortalSessions.Create(ctx, &stripe.BillingPortalSessionCreateParams{
		Customer:  stripe.String(ws.StripeCustomerID.String),
		ReturnURL: stripe.String(returnURL),
	})
	if err != nil {
		return "", err
	}
	return sess.URL, nil
}

// CancelSubscription cancels an active Stripe subscription immediately.
// Used during account deletion. No-op if stripeKey is not configured or
// subscriptionID is empty.
func (s *Service) CancelSubscription(ctx context.Context, subscriptionID string) error {
	if s.stripeSecretKey == "" || subscriptionID == "" {
		return nil
	}
	sc := stripe.NewClient(s.stripeSecretKey)
	_, err := sc.V1Subscriptions.Cancel(ctx, subscriptionID, &stripe.SubscriptionCancelParams{})
	if err != nil {
		return err
	}
	s.log.Info().Str("subscription_id", subscriptionID).Msg("subscription canceled")
	return nil
}

// HandleWebhook processes an incoming Stripe webhook event.
// Handles: customer.subscription.created/updated/deleted, invoice.payment_succeeded, invoice.payment_failed.
func (s *Service) HandleWebhook(ctx context.Context, payload []byte, signature string) error {
	if s.stripeWebhookSecret == "" {
		return ErrStripeDisabled
	}
	event, err := webhook.ConstructEventWithOptions(payload, signature, s.stripeWebhookSecret,
		webhook.ConstructEventOptions{IgnoreAPIVersionMismatch: true})
	if err != nil {
		return err
	}

	switch event.Type {
	case "payment_intent.payment_failed":
		return s.handlePaymentIntentFailed(ctx, event)
	case "invoice.payment_failed":
		return s.handlePaymentFailed(ctx, event)
	case "invoice.payment_succeeded":
		return s.handlePaymentSucceeded(ctx, event)
	case "customer.subscription.created":
		return s.handleSubscriptionCreated(ctx, event)
	case "customer.subscription.updated":
		return s.handleSubscriptionUpdated(ctx, event)
	case "customer.subscription.deleted":
		return s.handleSubscriptionDeleted(ctx, event)
	}
	return nil
}

// handleSubscriptionCreated records the subscription ID when a new subscription is created.
// Plan access is intentionally NOT granted here — it is granted only on invoice.payment_succeeded.
func (s *Service) handleSubscriptionCreated(ctx context.Context, event stripe.Event) error {
	var sub stripe.Subscription
	if err := json.Unmarshal(event.Data.Raw, &sub); err != nil {
		return err
	}
	if sub.Customer == nil || sub.ID == "" {
		return nil
	}
	s.log.Info().Msg("subscription created")
	return s.db.SetStripeSubscriptionID(ctx, queries.SetStripeSubscriptionIDParams{
		StripeCustomerID:     pgtype.Text{String: sub.Customer.ID, Valid: true},
		StripeSubscriptionID: pgtype.Text{String: sub.ID, Valid: true},
	})
}

// handlePaymentSucceeded grants plan access once Stripe confirms payment.
// This is the authoritative event for activating or renewing Pro/Org access.
func (s *Service) handlePaymentSucceeded(ctx context.Context, event stripe.Event) error {
	var inv stripe.Invoice
	if err := json.Unmarshal(event.Data.Raw, &inv); err != nil {
		return err
	}
	if inv.Customer == nil {
		return nil
	}
	ws, err := s.db.GetWorkspaceByStripeCustomerID(ctx, pgtype.Text{String: inv.Customer.ID, Valid: true})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		return err
	}

	plan := ws.Plan
	var periodEnd pgtype.Timestamptz
	if inv.Lines != nil && len(inv.Lines.Data) > 0 {
		line := inv.Lines.Data[0]
		if line.Pricing != nil && line.Pricing.PriceDetails != nil {
			if p, ok := s.planForPriceID(line.Pricing.PriceDetails.Price); ok {
				plan = p
			}
		}
		if line.Period != nil && line.Period.End > 0 {
			periodEnd = pgtype.Timestamptz{Time: time.Unix(line.Period.End, 0), Valid: true}
		}
	}

	subID := ""
	if inv.Parent != nil && inv.Parent.SubscriptionDetails != nil && inv.Parent.SubscriptionDetails.Subscription != nil {
		subID = inv.Parent.SubscriptionDetails.Subscription.ID
	}

	s.log.Info().Str("workspace_id", ws.ID).Str("plan", plan).Msg("payment succeeded")
	return s.db.UpdateWorkspacePlan(ctx, queries.UpdateWorkspacePlanParams{
		ID:                   ws.ID,
		Plan:                 plan,
		PlanStatus:           "active",
		PlanPeriodEnd:        periodEnd,
		StripeSubscriptionID: pgtype.Text{String: subID, Valid: subID != ""},
	})
}

func (s *Service) handleSubscriptionUpdated(ctx context.Context, event stripe.Event) error {
	var sub stripe.Subscription
	if err := json.Unmarshal(event.Data.Raw, &sub); err != nil {
		return err
	}
	if sub.Customer == nil {
		return nil
	}
	ws, err := s.db.GetWorkspaceByStripeCustomerID(ctx, pgtype.Text{String: sub.Customer.ID, Valid: true})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		return err
	}

	// Derive plan from the subscription's price ID; fall back to current plan if unrecognised.
	plan := ws.Plan
	if sub.Items != nil && len(sub.Items.Data) > 0 && sub.Items.Data[0].Price != nil {
		if p, ok := s.planForPriceID(sub.Items.Data[0].Price.ID); ok {
			plan = p
		}
	}

	var periodEnd pgtype.Timestamptz
	if sub.Items != nil && len(sub.Items.Data) > 0 && sub.Items.Data[0].CurrentPeriodEnd > 0 {
		periodEnd = pgtype.Timestamptz{Time: time.Unix(sub.Items.Data[0].CurrentPeriodEnd, 0), Valid: true}
	}
	status := normalisePlanStatus(string(sub.Status))
	if sub.CancelAtPeriodEnd && status == "active" {
		status = "canceling"
	}
	s.log.Info().Str("workspace_id", ws.ID).Str("plan", plan).Str("status", status).Bool("cancel_at_period_end", sub.CancelAtPeriodEnd).Msg("subscription updated")
	return s.db.UpdateWorkspacePlan(ctx, queries.UpdateWorkspacePlanParams{
		ID:                   ws.ID,
		Plan:                 plan,
		PlanStatus:           status,
		PlanPeriodEnd:        periodEnd,
		StripeSubscriptionID: pgtype.Text{String: sub.ID, Valid: sub.ID != ""},
	})
}

func (s *Service) handleSubscriptionDeleted(ctx context.Context, event stripe.Event) error {
	var sub stripe.Subscription
	if err := json.Unmarshal(event.Data.Raw, &sub); err != nil {
		return err
	}
	if sub.Customer == nil {
		return nil
	}
	ws, err := s.db.GetWorkspaceByStripeCustomerID(ctx, pgtype.Text{String: sub.Customer.ID, Valid: true})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		return err
	}
	s.log.Info().Str("workspace_id", ws.ID).Str("plan", ws.Plan).Msg("subscription deleted")
	return s.db.UpdateWorkspacePlan(ctx, queries.UpdateWorkspacePlanParams{
		ID:         ws.ID,
		Plan:       "free",
		PlanStatus: "canceled",
		// Clear the subscription ID once deleted.
		StripeSubscriptionID: pgtype.Text{},
	})
}

func (s *Service) handlePaymentFailed(ctx context.Context, event stripe.Event) error {
	var inv stripe.Invoice
	if err := json.Unmarshal(event.Data.Raw, &inv); err != nil {
		return err
	}
	if inv.Customer == nil {
		return nil
	}
	ws, err := s.db.GetWorkspaceByStripeCustomerID(ctx, pgtype.Text{String: inv.Customer.ID, Valid: true})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		return err
	}

	s.log.Warn().Str("workspace_id", ws.ID).Str("plan", ws.Plan).Msg("invoice payment failed")

	return s.db.UpdateWorkspacePlan(ctx, queries.UpdateWorkspacePlanParams{
		ID:                   ws.ID,
		Plan:                 ws.Plan,
		PlanStatus:           "past_due",
		StripeSubscriptionID: ws.StripeSubscriptionID,
	})
}

func (s *Service) handlePaymentIntentFailed(ctx context.Context, event stripe.Event) error {
	var pi stripe.PaymentIntent
	if err := json.Unmarshal(event.Data.Raw, &pi); err != nil {
		return err
	}

	ev := s.log.Warn()
	if pi.Customer != nil {
		ws, err := s.db.GetWorkspaceByStripeCustomerID(ctx, pgtype.Text{String: pi.Customer.ID, Valid: true})
		if err == nil {
			ev = ev.Str("workspace_id", ws.ID).Str("plan", ws.Plan)
		}
	}
	if pi.LastPaymentError != nil {
		ev = ev.Str("decline_code", string(pi.LastPaymentError.DeclineCode)).Str("error_code", string(pi.LastPaymentError.Code))
	}
	ev.Msg("payment intent failed")
	return nil
}

// normalisePlanStatus maps Stripe subscription statuses to our constrained set.
func normalisePlanStatus(stripeStatus string) string {
	switch stripeStatus {
	case "past_due":
		return "past_due"
	case "canceled", "cancelled", "unpaid":
		return "canceled"
	default:
		return "active"
	}
}

// priceIDForPlan returns the Stripe Price ID for the given plan name.
func (s *Service) priceIDForPlan(plan string) (string, error) {
	switch plan {
	case "pro":
		if s.stripePriceIDPro == "" {
			return "", ErrStripeDisabled
		}
		return s.stripePriceIDPro, nil
	case "org":
		if s.stripePriceIDOrg == "" {
			return "", ErrStripeDisabled
		}
		return s.stripePriceIDOrg, nil
	default:
		return "", ErrInvalidPlan
	}
}

// planForPriceID maps a Stripe Price ID back to a plan name.
func (s *Service) planForPriceID(priceID string) (string, bool) {
	if s.stripePriceIDPro != "" && priceID == s.stripePriceIDPro {
		return "pro", true
	}
	if s.stripePriceIDOrg != "" && priceID == s.stripePriceIDOrg {
		return "org", true
	}
	return "", false
}

// PlanMemberLimit returns the maximum number of members allowed for a plan.
// Returns -1 for unlimited.
func PlanMemberLimit(plan string) int64 {
	switch plan {
	case "free":
		return 2
	case "pro":
		return 10
	default:
		return -1
	}
}
