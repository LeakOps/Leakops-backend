package handlers

import (
	"time"

	"Leakops-backend/internal/models"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)


type AuthHandler struct {
	DB			*gorm.DB
	JWTSecret	string
}


func NewAuthHandler(db *gorm.DB, jwtSecret string) *AuthHandler {
	return &AuthHandler{DB: db, JWTSecret: jwtSecret}
}

type SignupRequest struct {
	Name		string		`json:"name"`
	Email		string		`json:"email"`
	Password 	string 		`json:"password"`
}


type LoginRequest struct {
	Email		string		`json:"email"`
	Password	string		`json:"password"`
}


// generateToken creates a signed JWT for the given user.
// userID is explicitly converted to string before going into claims,
// so that middleware can safely do claims["user_id"].(string) later.
func (h *AuthHandler) generateToken(userID uuid.UUID) (string, error) {
	claims := jwt.MapClaims{
		"user_id": userID.String(), // explicit string conversion
		"exp":     time.Now().Add(24 * time.Hour).Unix(),
		"iat":     time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(h.JWTSecret))
}

func (h *AuthHandler) Signup(c *fiber.Ctx) error {
    var req SignupRequest

    if err := c.BodyParser(&req); err != nil {
        return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
            "error": "invalid request body",
        })
    }

    if req.Name == "" || req.Email == "" || req.Password == "" {
        return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
            "error": "name, email and password are required",
        })
    }

    var existing models.User
    result := h.DB.Where("email = ?", req.Email).First(&existing)

    if result.RowsAffected > 0 {
        return c.Status(fiber.StatusConflict).JSON(fiber.Map{
            "error": "email already registered",
        })
    }

    hashedPassword, err := bcrypt.GenerateFromPassword(
        []byte(req.Password),
        bcrypt.DefaultCost,
    )
    if err != nil {
        return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
            "error": "failed to process password",
        })
    }

    user := models.User{
        Name:         req.Name,
        Email:        req.Email,
        PasswordHash: string(hashedPassword),
        Provider:     models.ProviderEmail,
    }

    if err := h.DB.Create(&user).Error; err != nil {
        return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
            "error": "failed to create user",
        })
    }

    token, err := h.generateToken(user.ID)
    if err != nil {
        return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
            "error": "failed to generate token",
        })
    }

    return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"token": token,
		"user": fiber.Map{
			"id":   	user.ID,
			"name": 	user.Name,
			"email": 	user.Email,
		},
	})
}


func (h* AuthHandler) login(c *fiber.Ctx) error {
	var req LoginRequest

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request body",
		})
	}

	if req.Email == "" || req.Password == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "invalid email or password",
		})
	}

	var user models.User
	result := h.DB.Where("email = ? AND provider = ?", req.Email, models.ProviderEmail).First(&user)

	if result.RowsAffected == 0 {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "invalid email or password",
		})
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash),  []byte(req.Password)); err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "invalid email or password",
		})
	}

	token, err := h.generateToken(user.ID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to generate token",
		})
	}

	return c.JSON(fiber.Map{
		"token": token,
		"user": fiber.Map{
			"id": user.ID,
			"name": user.Name,
			"email": user.Email,
		},
	})
}

