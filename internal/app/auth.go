package app

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"tankobon/internal/repository"
)

var jwtSecret string

func init() {
	jwtSecret = os.Getenv("TANKOBON_JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = "change-me-in-production"
	}
}

type Claims struct {
	UserID   uint   `json:"user_id"`
	Username string `json:"username"`
	jwt.RegisteredClaims
}

type Tokens struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
}

type AuthToken struct {
	Username string
	UserID   uint
}

func Login(db *gorm.DB) fiber.Handler {
	return func(c fiber.Ctx) error {
		var req struct {
			Username string `json:"username"`
			Password string `json:"password"`
		}
		if err := c.Bind().JSON(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
		}

		user, err := repository.GetUser(db, req.Username)
		if err != nil || !VerifyPassword(user.Password, req.Password) {
			return c.Status(401).JSON(fiber.Map{"error": "invalid credentials"})
		}

		tokens, err := GenerateTokens(user.ID, user.Username)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "failed to generate token"})
		}

		return c.JSON(fiber.Map{
			"username":     user.Username,
			"accessToken":  tokens.AccessToken,
			"refreshToken": tokens.RefreshToken,
		})
	}
}

func Register(db *gorm.DB) fiber.Handler {
	return func(c fiber.Ctx) error {
		// TODO: i do want to allow multiple accounts, but not yet.
		if exists, err := repository.AccountExists(db); err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "internal server error"})
		} else if exists {
			return c.Status(403).JSON(fiber.Map{"error": "only 1 user allowed"})
		}

		var req struct {
			Username string `json:"username"`
			Password string `json:"password"`
		}
		if err := c.Bind().JSON(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
		}

		if req.Username == "" || req.Password == "" {
			return c.Status(400).JSON(fiber.Map{"error": "username and password are required"})
		}

		hash, err := HashPassword(req.Password)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "failed to hash password"})
		}

		user, err := repository.CreateUser(db, req.Username, hash)
		if err != nil {
			return c.Status(409).JSON(fiber.Map{"error": "username already taken"})
		}

		tokens, err := GenerateTokens(user.ID, user.Username)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "failed to generate token"})
		}

		return c.Status(201).JSON(fiber.Map{
			"username":     user.Username,
			"accessToken":  tokens.AccessToken,
			"refreshToken": tokens.RefreshToken,
		})
	}
}

func RefreshToken(db *gorm.DB) fiber.Handler {
	return func(c fiber.Ctx) error {
		auth := c.Get("Authorization")
		if auth == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "missing token"})
		}

		auth = strings.TrimPrefix(auth, "Bearer ")

		claims, err := VerifyToken(auth)
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid token"})
		}

		if _, err := repository.GetUser(db, claims.Username); err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "user not found"})
		}

		tokens, err := GenerateTokens(claims.UserID, claims.Username)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "failed to generate token"})
		}

		return c.JSON(fiber.Map{
			"username":     claims.Username,
			"accessToken":  tokens.AccessToken,
			"refreshToken": tokens.RefreshToken,
		})
	}
}

func Exists(db *gorm.DB) fiber.Handler {
	return func(c fiber.Ctx) error {
		exists, err := repository.AccountExists(db)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "internal server error"})
		}
		return c.JSON(exists)
	}
}

func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}
	return string(hash), nil
}

func VerifyPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

func GenerateTokens(userID uint, username string) (Tokens, error) {
	accessToken, err := genToken(userID, username, time.Minute*15)
	if err != nil {
		return Tokens{}, fmt.Errorf("generate access token: %w", err)
	}
	refreshToken, err := genToken(userID, username, time.Hour*24*7)
	if err != nil {
		return Tokens{}, fmt.Errorf("generate refresh token: %w", err)
	}
	return Tokens{AccessToken: accessToken, RefreshToken: refreshToken}, nil
}

func genToken(userID uint, username string, expire time.Duration) (string, error) {
	claims := Claims{
		UserID:   userID,
		Username: username,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(expire)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(jwtSecret))
}

func VerifyToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(jwtSecret), nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}
	return claims, nil
}

func Middleware() fiber.Handler {
	return func(c fiber.Ctx) error {
		auth := c.Get("Authorization")
		if auth == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "missing token"})
		}

		auth = strings.TrimPrefix(auth, "Bearer ")

		claims, err := VerifyToken(auth)
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid token"})
		}

		c.Locals("auth", &AuthToken{Username: claims.Username, UserID: claims.UserID})
		return c.Next()
	}
}
