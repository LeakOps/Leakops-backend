package handlers

import(
	"Leakops-backend/internal/email"

	"github.com/gofiber/fiber/v2"
)


type FeedbackHandler struct {
	DunningSvc *email.DunningService
}


func NewFeedbackHandler(dunningSvc *email.DunningService) *FeedbackHandler {
	return &FeedbackHandler{DunningSvc: dunningSvc}
}

func (h *FeedbackHandler) SubmitFeedback(c *fiber.Ctx) error {
	var req struct {
		Name    string `json:"name"`
		Email   string `json:"email"`
		Message string `json:"message"`
	}

	if err := c.BodyParser(&req); err != nil || req.Message == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "message is required",
		})
	}

	if err := h.DunningSvc.SendFeedbackNotification(req.Name, req.Email, req.Message); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to submit feedback",
		})
	}

	return c.JSON(fiber.Map{"message": "thank you for your feedback"})
}
