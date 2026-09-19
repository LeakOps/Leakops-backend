package handlers

import(
	"fmt"

	"Leakops-backend/internal/models"
	"Leakops-backend/internal/services"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"gorm.io/gorm"
)


type ProfileHandler struct {
	DB		*gorm.DB
	Storage	*services.StorageService
}


func NewProfileHandler(db *gorm.DB, storage *services.StorageService) *ProfileHandler {
	return &ProfileHandler{DB: db, Storage: storage}
}

func (h *ProfileHandler) UploadProfilePicture(c *fiber.Ctx) error {
	userIDStr := c.Locals("userID").(string)
	userID, err := uuid.Parse(userIDStr)

	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "invalid user",
		})
	}

	fileHeader, err := c.FormFile("image")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "image file is required",
		})
	}

	if fileHeader.Size > 2*1024*1024 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "image must be under 2MB",
		})
	}

	contentType := fileHeader.Header.Get("Content-Type")
	allowed := map[string]bool{"image/jpeg": true, "image/png": true, "image/webp": true}
	if !allowed[contentType] {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "only jpeg, png, webp allowed",
		})
	}

	file, err := fileHeader.Open()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to read file",
		})
	}
	defer file.Close()

	key := fmt.Sprintf("%s%s", userID.String(), extFromContentType(contentType))

	publicURL, err := h.Storage.UploadFile(c.Context(), key, file, contentType)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to upload image",
		})
	}

	h.DB.Model(&models.User{}).Where("id = ?", userID).Update("profile_picture_url", publicURL)

	return c.JSON(fiber.Map{
		"profile_picture_url": publicURL,
	})
}


func extFromContentType(contentType string) string {
	switch contentType {
	case "image/jpeg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/webp":
		return ".webp"
	default:
		return ""
	}
}

