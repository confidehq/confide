package billing

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog/log"

	mw "github.com/phantompunk/confide/internal/middleware"
	"github.com/phantompunk/confide/internal/permission"
	"github.com/phantompunk/confide/internal/workspace"
)

// Handler returns the billing sub-router mounted at /api/workspaces/{workspaceId}/billing.
// All routes require authentication (caller must be workspace owner).
func Handler(svc *Service) http.Handler {
	r := chi.NewRouter()
	r.Get("/", getBillingInfo(svc))
	r.Post("/subscribe", subscribe(svc))
	r.Post("/portal", portal(svc))
	return r
}

// UsageHandler returns the usage sub-router mounted at /api/workspaces/{workspaceId}/usage.
// Viewer+ access — any workspace member may retrieve usage.
func UsageHandler(svc *Service, wsSvc *workspace.Service) http.Handler {
	r := chi.NewRouter()
	r.Get("/", getUsage(svc, wsSvc))
	return r
}

// WebhookHandler returns the public Stripe webhook handler.
// Mounted at /api/stripe/webhook — no authentication.
func WebhookHandler(svc *Service) http.HandlerFunc {
	return stripeWebhook(svc)
}

// ─── Workspace billing handlers ───────────────────────────────────────────────

func getBillingInfo(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		callerRole := permission.WorkspaceRole(r.Context())
		workspaceID := chi.URLParam(r, "workspaceId")

		info, err := svc.GetInfo(r.Context(), workspaceID, callerRole)
		if err != nil {
			if errors.Is(err, ErrForbidden) {
				writeError(w, http.StatusForbidden, "forbidden", "owner role required")
				return
			}
			if errors.Is(err, ErrNotFound) {
				writeError(w, http.StatusNotFound, "not_found", "workspace not found")
				return
			}
			writeError(w, http.StatusInternalServerError, "internal", "failed to get billing info")
			return
		}

		resp := map[string]any{
			"plan":                 info.Plan,
			"planStatus":           info.PlanStatus,
			"memberCount":          info.MemberCount,
			"formCount":            info.FormCount,
			"monthlyResponseCount": info.MonthlyResponseCount,
			"hasStripeCustomer":    info.HasStripeCustomer,
		}
		if info.PlanPeriodEnd != nil {
			resp["planPeriodEnd"] = info.PlanPeriodEnd.Format("2006-01-02T15:04:05Z07:00")
		}
		writeJSON(w, http.StatusOK, resp)
	}
}

func subscribe(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		callerRole := permission.WorkspaceRole(r.Context())
		workspaceID := chi.URLParam(r, "workspaceId")

		var req struct {
			Plan       string `json:"plan"`
			SuccessURL string `json:"successUrl"`
			CancelURL  string `json:"cancelUrl"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", "invalid request body")
			return
		}
		if req.Plan == "" || req.SuccessURL == "" || req.CancelURL == "" {
			writeError(w, http.StatusBadRequest, "invalid_field", "plan, successUrl and cancelUrl are required")
			return
		}

		url, err := svc.Subscribe(r.Context(), workspaceID, callerRole, req.Plan, req.SuccessURL, req.CancelURL)
		if err != nil {
			if errors.Is(err, ErrForbidden) {
				writeError(w, http.StatusForbidden, "forbidden", "owner role required")
				return
			}
			if errors.Is(err, ErrNotFound) {
				writeError(w, http.StatusNotFound, "not_found", "workspace not found")
				return
			}
			if errors.Is(err, ErrStripeDisabled) {
				writeError(w, http.StatusServiceUnavailable, "stripe_disabled", "billing not configured")
				return
			}
			if errors.Is(err, ErrInvalidPlan) {
				writeError(w, http.StatusBadRequest, "invalid_plan", "invalid plan")
				return
			}
			writeError(w, http.StatusInternalServerError, "internal", "failed to create checkout session")
			return
		}

		writeJSON(w, http.StatusOK, map[string]string{"url": url})
	}
}

func portal(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		callerRole := permission.WorkspaceRole(r.Context())
		workspaceID := chi.URLParam(r, "workspaceId")

		var req struct {
			ReturnURL string `json:"returnUrl"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", "invalid request body")
			return
		}
		if req.ReturnURL == "" {
			writeError(w, http.StatusBadRequest, "invalid_field", "returnUrl is required")
			return
		}

		url, err := svc.Portal(r.Context(), workspaceID, callerRole, req.ReturnURL)
		if err != nil {
			if errors.Is(err, ErrForbidden) {
				writeError(w, http.StatusForbidden, "forbidden", "owner role required")
				return
			}
			if errors.Is(err, ErrNotFound) {
				writeError(w, http.StatusNotFound, "not_found", "workspace not found")
				return
			}
			if errors.Is(err, ErrStripeDisabled) {
				writeError(w, http.StatusServiceUnavailable, "stripe_disabled", "billing not configured")
				return
			}
			if errors.Is(err, ErrNoCustomer) {
				writeError(w, http.StatusConflict, "no_customer", "workspace has never subscribed")
				return
			}
			writeError(w, http.StatusInternalServerError, "internal", "failed to create portal session")
			return
		}

		writeJSON(w, http.StatusOK, map[string]string{"url": url})
	}
}

// ─── Usage handler ───────────────────────────────────────────────────────────

func getUsage(svc *Service, wsSvc *workspace.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		workspaceID := chi.URLParam(r, "workspaceId")
		accountID := mw.AccountID(r.Context())

		if err := wsSvc.ValidateMember(r.Context(), workspaceID, accountID); err != nil {
			writeError(w, http.StatusForbidden, "forbidden", "workspace member required")
			return
		}

		usage, err := svc.GetUsage(r.Context(), workspaceID)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				writeError(w, http.StatusNotFound, "not_found", "workspace not found")
				return
			}
			writeError(w, http.StatusInternalServerError, "internal", "failed to get usage")
			return
		}

		writeJSON(w, http.StatusOK, usage)
	}
}

// ─── Webhook handler ──────────────────────────────────────────────────────────

func stripeWebhook(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		payload, err := io.ReadAll(io.LimitReader(r.Body, 65536))
		if err != nil {
			writeError(w, http.StatusBadRequest, "read_error", "failed to read request body")
			return
		}

		sig := r.Header.Get("Stripe-Signature")
		if err := svc.HandleWebhook(r.Context(), payload, sig); err != nil {
			if errors.Is(err, ErrStripeDisabled) {
				writeError(w, http.StatusServiceUnavailable, "stripe_disabled", "webhook not configured")
				return
			}
			// Log the real error server-side; return a generic message to the caller
			// so internal Stripe SDK details are not leaked in the response body.
			log.Error().Err(err).Msg("billing: stripe webhook error")
			writeError(w, http.StatusBadRequest, "webhook_error", "invalid webhook payload")
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]string{"code": code, "message": message})
}
