package handlers

import (
	"github.com/gofiber/fiber/v2"

	"truyenm/backend/internal/models"
	"truyenm/backend/internal/utils"
)

// ─── Support Ticket Handlers ──────────────────────────────────────────────────

type CreateTicketRequest struct {
	Subject string `json:"subject"`
	Body    string `json:"body"`
}

type ReplyTicketRequest struct {
	Body string `json:"body"`
}

func (h *Handler) CreateTicket(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(uint)

	var req CreateTicketRequest
	if err := c.BodyParser(&req); err != nil || req.Subject == "" || req.Body == "" {
		return fiber.NewError(fiber.StatusBadRequest, "subject and body required")
	}

	ticket := models.SupportTicket{
		UserID:   userID,
		Subject:  req.Subject,
		Status:   models.TicketStatusOpen,
	}
	if err := h.DB.Create(&ticket).Error; err != nil {
		return fiber.ErrInternalServerError
	}

	// Create first message
	msg := models.TicketMessage{
		TicketID:     ticket.ID,
		UserID:       userID,
		Body:         req.Body,
		IsAdminReply: false,
	}
	h.DB.Create(&msg)

	return c.Status(fiber.StatusCreated).JSON(ticket)
}

func (h *Handler) ListTickets(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(uint)
	role, _ := c.Locals("role").(string)
	page, limit := utils.ParsePagination(c, 1, 20)
	offset := (page - 1) * limit

	isAdmin := role == models.RoleOwner || role == models.RoleAdmin ||
		role == models.RoleStaff || role == models.RoleModerator

	db := h.DB.Model(&models.SupportTicket{}).Preload("User")
	var total int64

	if !isAdmin {
		db = db.Where("user_id = ?", userID)
	}

	// Filters (admin only)
	if isAdmin {
		if status := c.Query("status"); status != "" {
			db = db.Where("status = ?", status)
		}
	}

	db.Count(&total)
	var tickets []models.SupportTicket
	if err := db.Limit(limit).Offset(offset).Order("updated_at desc").Find(&tickets).Error; err != nil {
		return fiber.ErrInternalServerError
	}

	return c.JSON(fiber.Map{"page": page, "limit": limit, "total": total, "data": tickets})
}

func (h *Handler) GetTicket(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(uint)
	role, _ := c.Locals("role").(string)

	id, err := c.ParamsInt("id")
	if err != nil || id <= 0 {
		return fiber.ErrBadRequest
	}

	var ticket models.SupportTicket
	if err := h.DB.Preload("User").Preload("Messages.User").First(&ticket, id).Error; err != nil {
		return fiber.ErrNotFound
	}

	isAdmin := role == models.RoleOwner || role == models.RoleAdmin ||
		role == models.RoleStaff || role == models.RoleModerator

	if !isAdmin && ticket.UserID != userID {
		return fiber.ErrForbidden
	}

	return c.JSON(ticket)
}

func (h *Handler) ReplyTicket(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(uint)
	role, _ := c.Locals("role").(string)

	id, err := c.ParamsInt("id")
	if err != nil || id <= 0 {
		return fiber.ErrBadRequest
	}

	var req ReplyTicketRequest
	if err := c.BodyParser(&req); err != nil || req.Body == "" {
		return fiber.NewError(fiber.StatusBadRequest, "body required")
	}

	var ticket models.SupportTicket
	if err := h.DB.First(&ticket, id).Error; err != nil {
		return fiber.ErrNotFound
	}

	isAdmin := role == models.RoleOwner || role == models.RoleAdmin ||
		role == models.RoleStaff || role == models.RoleModerator

	if !isAdmin && ticket.UserID != userID {
		return fiber.ErrForbidden
	}
	if ticket.Status == models.TicketStatusClosed {
		return fiber.NewError(fiber.StatusBadRequest, "ticket is closed")
	}

	msg := models.TicketMessage{
		TicketID:     ticket.ID,
		UserID:       userID,
		Body:         req.Body,
		IsAdminReply: isAdmin,
	}
	if err := h.DB.Create(&msg).Error; err != nil {
		return fiber.ErrInternalServerError
	}

	// Update ticket status
	if isAdmin {
		h.DB.Model(&ticket).Update("status", models.TicketStatusOpen)
	} else {
		h.DB.Model(&ticket).Update("status", models.TicketStatusPending)
	}

	return c.JSON(msg)
}

func (h *Handler) CloseTicket(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(uint)
	role, _ := c.Locals("role").(string)

	id, err := c.ParamsInt("id")
	if err != nil || id <= 0 {
		return fiber.ErrBadRequest
	}

	var ticket models.SupportTicket
	if err := h.DB.First(&ticket, id).Error; err != nil {
		return fiber.ErrNotFound
	}

	isAdmin := role == models.RoleOwner || role == models.RoleAdmin ||
		role == models.RoleStaff || role == models.RoleModerator

	if !isAdmin && ticket.UserID != userID {
		return fiber.ErrForbidden
	}

	if err := h.DB.Model(&models.SupportTicket{}).Where("id = ?", id).
		Update("status", models.TicketStatusClosed).Error; err != nil {
		return fiber.ErrInternalServerError
	}

	return c.JSON(fiber.Map{"ok": true})
}
