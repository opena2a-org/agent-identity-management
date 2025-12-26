package handlers

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ===========================
// NewPublicRegistrationHandler Tests
// ===========================

func TestNewPublicRegistrationHandler_NilDeps(t *testing.T) {
	handler := NewPublicRegistrationHandler(nil, nil, nil)
	assert.NotNil(t, handler)
}

// ===========================
// PublicRegistrationHandler.RegisterUser Tests
// ===========================

func TestPublicRegistrationHandler_RegisterUser_InvalidJSON(t *testing.T) {
	handler := &PublicRegistrationHandler{}
	app := fiber.New()
	app.Post("/public/register", handler.RegisterUser)

	req := httptest.NewRequest("POST", "/public/register", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestPublicRegistrationHandler_RegisterUser_EmptyFields(t *testing.T) {
	handler := &PublicRegistrationHandler{}
	app := fiber.New()
	app.Post("/public/register", handler.RegisterUser)

	body := `{"email":"","firstName":"","lastName":"","password":""}`
	req := httptest.NewRequest("POST", "/public/register", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestPublicRegistrationHandler_RegisterUser_MissingEmail(t *testing.T) {
	handler := &PublicRegistrationHandler{}
	app := fiber.New()
	app.Post("/public/register", handler.RegisterUser)

	body := `{"firstName":"John","lastName":"Doe","password":"password123"}`
	req := httptest.NewRequest("POST", "/public/register", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestPublicRegistrationHandler_RegisterUser_MissingPassword(t *testing.T) {
	handler := &PublicRegistrationHandler{}
	app := fiber.New()
	app.Post("/public/register", handler.RegisterUser)

	body := `{"email":"test@example.com","firstName":"John","lastName":"Doe"}`
	req := httptest.NewRequest("POST", "/public/register", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

// ===========================
// PublicRegistrationHandler.CheckRegistrationStatus Tests
// ===========================

func TestPublicRegistrationHandler_CheckRegistrationStatus_InvalidID(t *testing.T) {
	handler := &PublicRegistrationHandler{}
	app := fiber.New()
	app.Get("/public/register/:requestId/status", handler.CheckRegistrationStatus)

	req := httptest.NewRequest("GET", "/public/register/not-a-uuid/status", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

// ===========================
// PublicRegistrationHandler.Login Tests
// ===========================

func TestPublicRegistrationHandler_Login_InvalidJSON(t *testing.T) {
	handler := &PublicRegistrationHandler{}
	app := fiber.New()
	app.Post("/public/login", handler.Login)

	req := httptest.NewRequest("POST", "/public/login", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestPublicRegistrationHandler_Login_EmptyEmail(t *testing.T) {
	handler := &PublicRegistrationHandler{}
	app := fiber.New()
	app.Post("/public/login", handler.Login)

	body := `{"email":"","password":"password123"}`
	req := httptest.NewRequest("POST", "/public/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestPublicRegistrationHandler_Login_EmptyPassword(t *testing.T) {
	handler := &PublicRegistrationHandler{}
	app := fiber.New()
	app.Post("/public/login", handler.Login)

	body := `{"email":"test@example.com","password":""}`
	req := httptest.NewRequest("POST", "/public/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

// ===========================
// PublicRegistrationHandler.RequestAccess Tests
// ===========================

func TestPublicRegistrationHandler_RequestAccess_InvalidJSON(t *testing.T) {
	handler := &PublicRegistrationHandler{}
	app := fiber.New()
	app.Post("/public/request-access", handler.RequestAccess)

	req := httptest.NewRequest("POST", "/public/request-access", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestPublicRegistrationHandler_RequestAccess_EmptyEmail(t *testing.T) {
	handler := &PublicRegistrationHandler{}
	app := fiber.New()
	app.Post("/public/request-access", handler.RequestAccess)

	body := `{"email":"","fullName":"John Doe","reason":"This is a valid reason"}`
	req := httptest.NewRequest("POST", "/public/request-access", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestPublicRegistrationHandler_RequestAccess_EmptyFullName(t *testing.T) {
	handler := &PublicRegistrationHandler{}
	app := fiber.New()
	app.Post("/public/request-access", handler.RequestAccess)

	body := `{"email":"test@example.com","fullName":"","reason":"This is a valid reason"}`
	req := httptest.NewRequest("POST", "/public/request-access", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestPublicRegistrationHandler_RequestAccess_EmptyReason(t *testing.T) {
	handler := &PublicRegistrationHandler{}
	app := fiber.New()
	app.Post("/public/request-access", handler.RequestAccess)

	body := `{"email":"test@example.com","fullName":"John Doe","reason":""}`
	req := httptest.NewRequest("POST", "/public/request-access", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestPublicRegistrationHandler_RequestAccess_ShortReason(t *testing.T) {
	handler := &PublicRegistrationHandler{}
	app := fiber.New()
	app.Post("/public/request-access", handler.RequestAccess)

	body := `{"email":"test@example.com","fullName":"John Doe","reason":"short"}`
	req := httptest.NewRequest("POST", "/public/request-access", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

// ===========================
// PublicRegistrationHandler.ChangePassword Tests
// ===========================

func TestPublicRegistrationHandler_ChangePassword_InvalidJSON(t *testing.T) {
	handler := &PublicRegistrationHandler{}
	app := fiber.New()
	app.Post("/public/change-password", handler.ChangePassword)

	req := httptest.NewRequest("POST", "/public/change-password", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestPublicRegistrationHandler_ChangePassword_EmptyFields(t *testing.T) {
	handler := &PublicRegistrationHandler{}
	app := fiber.New()
	app.Post("/public/change-password", handler.ChangePassword)

	body := `{"email":"","oldPassword":"","newPassword":""}`
	req := httptest.NewRequest("POST", "/public/change-password", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestPublicRegistrationHandler_ChangePassword_ShortNewPassword(t *testing.T) {
	handler := &PublicRegistrationHandler{}
	app := fiber.New()
	app.Post("/public/change-password", handler.ChangePassword)

	body := `{"email":"test@example.com","oldPassword":"oldpassword123","newPassword":"short"}`
	req := httptest.NewRequest("POST", "/public/change-password", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestPublicRegistrationHandler_ChangePassword_InvalidUserID(t *testing.T) {
	handler := &PublicRegistrationHandler{}
	app := fiber.New()
	app.Post("/public/change-password", handler.ChangePassword)

	body := `{"userId":"not-a-uuid","currentPassword":"old","newPassword":"new"}`
	req := httptest.NewRequest("POST", "/public/change-password", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

// ===========================
// PublicRegistrationHandler.ForgotPassword Tests
// ===========================

func TestPublicRegistrationHandler_ForgotPassword_InvalidJSON(t *testing.T) {
	handler := &PublicRegistrationHandler{}
	app := fiber.New()
	app.Post("/public/forgot-password", handler.ForgotPassword)

	req := httptest.NewRequest("POST", "/public/forgot-password", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestPublicRegistrationHandler_ForgotPassword_EmptyEmail(t *testing.T) {
	handler := &PublicRegistrationHandler{}
	app := fiber.New()
	app.Post("/public/forgot-password", handler.ForgotPassword)

	body := `{"email":""}`
	req := httptest.NewRequest("POST", "/public/forgot-password", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

// ===========================
// PublicRegistrationHandler.ResetPassword Tests
// ===========================

func TestPublicRegistrationHandler_ResetPassword_InvalidJSON(t *testing.T) {
	handler := &PublicRegistrationHandler{}
	app := fiber.New()
	app.Post("/public/reset-password", handler.ResetPassword)

	req := httptest.NewRequest("POST", "/public/reset-password", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestPublicRegistrationHandler_ResetPassword_EmptyFields(t *testing.T) {
	handler := &PublicRegistrationHandler{}
	app := fiber.New()
	app.Post("/public/reset-password", handler.ResetPassword)

	body := `{"resetToken":"","newPassword":"","confirmPassword":""}`
	req := httptest.NewRequest("POST", "/public/reset-password", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestPublicRegistrationHandler_ResetPassword_PasswordMismatch(t *testing.T) {
	handler := &PublicRegistrationHandler{}
	app := fiber.New()
	app.Post("/public/reset-password", handler.ResetPassword)

	body := `{"resetToken":"valid-token","newPassword":"password123","confirmPassword":"different456"}`
	req := httptest.NewRequest("POST", "/public/reset-password", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestPublicRegistrationHandler_ResetPassword_ShortPassword(t *testing.T) {
	handler := &PublicRegistrationHandler{}
	app := fiber.New()
	app.Post("/public/reset-password", handler.ResetPassword)

	body := `{"resetToken":"valid-token","newPassword":"short","confirmPassword":"short"}`
	req := httptest.NewRequest("POST", "/public/reset-password", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

// ===========================
// PublicRegistrationHandler.RegisterRoutes Tests
// ===========================

func TestPublicRegistrationHandler_RegisterRoutes(t *testing.T) {
	handler := &PublicRegistrationHandler{}
	app := fiber.New()

	// Should not panic with nil handler
	handler.RegisterRoutes(app)

	// Verify routes are registered by making a request
	req := httptest.NewRequest("POST", "/api/v1/public/register", strings.NewReader("{}"))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Should get 400 for empty fields, not 404
	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

// ===========================
// Struct Tests
// ===========================

func TestRegisterUserRequest_Struct(t *testing.T) {
	req := RegisterUserRequest{
		Email:     "test@example.com",
		FirstName: "John",
		LastName:  "Doe",
		Password:  "securepassword123",
	}

	assert.Equal(t, "test@example.com", req.Email)
	assert.Equal(t, "John", req.FirstName)
	assert.Equal(t, "Doe", req.LastName)
	assert.Equal(t, "securepassword123", req.Password)
}

func TestRegisterUserRequest_EmptyStruct(t *testing.T) {
	req := RegisterUserRequest{}

	assert.Empty(t, req.Email)
	assert.Empty(t, req.FirstName)
	assert.Empty(t, req.LastName)
	assert.Empty(t, req.Password)
}

func TestRegisterUserResponse_Struct(t *testing.T) {
	resp := RegisterUserResponse{
		Success:             true,
		Message:             "Registration submitted",
		RegistrationRequest: nil,
	}

	assert.True(t, resp.Success)
	assert.Equal(t, "Registration submitted", resp.Message)
	assert.Nil(t, resp.RegistrationRequest)
}

func TestLoginRequest_Struct(t *testing.T) {
	req := LoginRequest{
		Email:    "user@example.com",
		Password: "mypassword",
	}

	assert.Equal(t, "user@example.com", req.Email)
	assert.Equal(t, "mypassword", req.Password)
}

func TestLoginResponse_Struct(t *testing.T) {
	accessToken := "access-token-123"
	refreshToken := "refresh-token-456"
	resp := LoginResponse{
		Success:                true,
		Message:                "Login successful",
		User:                   nil,
		AccessToken:            &accessToken,
		RefreshToken:           &refreshToken,
		IsApproved:             true,
		RequiresPasswordChange: false,
	}

	assert.True(t, resp.Success)
	assert.Equal(t, "Login successful", resp.Message)
	assert.Nil(t, resp.User)
	assert.NotNil(t, resp.AccessToken)
	assert.Equal(t, "access-token-123", *resp.AccessToken)
	assert.NotNil(t, resp.RefreshToken)
	assert.Equal(t, "refresh-token-456", *resp.RefreshToken)
	assert.True(t, resp.IsApproved)
	assert.False(t, resp.RequiresPasswordChange)
}

func TestLoginResponse_EmptyStruct(t *testing.T) {
	resp := LoginResponse{}

	assert.False(t, resp.Success)
	assert.Empty(t, resp.Message)
	assert.Nil(t, resp.User)
	assert.Nil(t, resp.AccessToken)
	assert.Nil(t, resp.RefreshToken)
	assert.False(t, resp.IsApproved)
	assert.False(t, resp.RequiresPasswordChange)
}

func TestChangePasswordRequest_Struct(t *testing.T) {
	req := ChangePasswordRequest{
		Email:       "user@example.com",
		OldPassword: "oldpass123",
		NewPassword: "newpass456",
	}

	assert.Equal(t, "user@example.com", req.Email)
	assert.Equal(t, "oldpass123", req.OldPassword)
	assert.Equal(t, "newpass456", req.NewPassword)
}

func TestRequestAccessRequest_Struct(t *testing.T) {
	orgName := "Acme Corp"
	req := RequestAccessRequest{
		Email:            "user@example.com",
		FullName:         "John Doe",
		OrganizationName: &orgName,
		Reason:           "I need access for project work",
	}

	assert.Equal(t, "user@example.com", req.Email)
	assert.Equal(t, "John Doe", req.FullName)
	assert.NotNil(t, req.OrganizationName)
	assert.Equal(t, "Acme Corp", *req.OrganizationName)
	assert.Equal(t, "I need access for project work", req.Reason)
}

func TestRequestAccessRequest_NilOrganization(t *testing.T) {
	req := RequestAccessRequest{
		Email:            "user@example.com",
		FullName:         "Jane Doe",
		OrganizationName: nil,
		Reason:           "Personal project",
	}

	assert.Nil(t, req.OrganizationName)
}

func TestRequestAccessResponse_Struct(t *testing.T) {
	resp := RequestAccessResponse{
		Success: true,
		Message: "Access request submitted",
		Status:  "pending",
	}

	assert.True(t, resp.Success)
	assert.Equal(t, "Access request submitted", resp.Message)
	assert.Equal(t, "pending", resp.Status)
}

func TestForgotPasswordRequest_Struct(t *testing.T) {
	req := ForgotPasswordRequest{
		Email: "forgot@example.com",
	}

	assert.Equal(t, "forgot@example.com", req.Email)
}

func TestForgotPasswordResponse_Struct(t *testing.T) {
	resp := ForgotPasswordResponse{
		Success: true,
		Message: "Password reset email sent",
	}

	assert.True(t, resp.Success)
	assert.Equal(t, "Password reset email sent", resp.Message)
}

func TestResetPasswordRequest_Struct(t *testing.T) {
	req := ResetPasswordRequest{
		ResetToken:      "reset-token-abc123",
		NewPassword:     "newSecurePassword",
		ConfirmPassword: "newSecurePassword",
	}

	assert.Equal(t, "reset-token-abc123", req.ResetToken)
	assert.Equal(t, "newSecurePassword", req.NewPassword)
	assert.Equal(t, "newSecurePassword", req.ConfirmPassword)
}

func TestResetPasswordRequest_MismatchedPasswords(t *testing.T) {
	req := ResetPasswordRequest{
		ResetToken:      "reset-token-abc123",
		NewPassword:     "password1",
		ConfirmPassword: "password2",
	}

	assert.NotEqual(t, req.NewPassword, req.ConfirmPassword)
}

func TestResetPasswordResponse_Struct(t *testing.T) {
	resp := ResetPasswordResponse{
		Success: true,
		Message: "Password reset successfully",
	}

	assert.True(t, resp.Success)
	assert.Equal(t, "Password reset successfully", resp.Message)
}
