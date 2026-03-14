package models

import "time"

const (
	RoleOwner     = "owner"
	RoleUser      = "user"
	RoleAdmin     = "admin"
	RoleStaff     = "staff"
	RoleModerator = "moderator"

	PointReasonRead       = "read_chapter"
	PointReasonDailyLogin = "daily_login"
	PointReasonComment    = "comment"
	PointReasonManual     = "manual"

	TicketStatusOpen    = "open"
	TicketStatusPending = "pending"
	TicketStatusClosed  = "closed"

	CrawlerModeFullSync = 1
	CrawlerModeDaily    = 2
	CrawlerModeSchedule = 3

	CrawlerStatusRunning = "running"
	CrawlerStatusSuccess = "success"
	CrawlerStatusFailed  = "failed"
)

type User struct {
	ID           uint       `gorm:"primaryKey" json:"id"`
	Username     string     `gorm:"size:50;unique;not null" json:"username"`
	Email        string     `gorm:"size:120;unique;not null" json:"email"`
	PasswordHash string     `gorm:"size:255;not null" json:"-"`
	Points       int        `gorm:"not null;default:0" json:"points"`
	Role         string     `gorm:"size:20;not null;default:user" json:"role"`
	IsBanned     bool       `gorm:"not null;default:false" json:"is_banned"`
	BannedAt     *time.Time `json:"banned_at,omitempty"`
	BanReason    string     `gorm:"size:255" json:"ban_reason,omitempty"`
	BanExpiresAt *time.Time `json:"ban_expires_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

type Genre struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Name      string    `gorm:"size:100;unique;not null" json:"name"`
	Slug      string    `gorm:"size:100;unique;not null" json:"slug"`
	IsActive  bool      `gorm:"not null;default:true" json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Comic struct {
	ID                     uint       `gorm:"primaryKey" json:"id"`
	Title                  string     `gorm:"size:200;not null" json:"title"`
	Slug                   string     `gorm:"size:220;uniqueIndex" json:"slug,omitempty"`
	Author                 string     `gorm:"size:120" json:"author,omitempty"`
	Description            string     `gorm:"type:text" json:"description,omitempty"`
	Cover                  string     `gorm:"type:text" json:"cover,omitempty"`
	Views                  int        `gorm:"not null;default:0" json:"views"`
	ContentType            string     `gorm:"size:20;not null;default:comic" json:"content_type"`
	Status                 string     `gorm:"size:20;not null;default:ongoing" json:"status"`
	CrawlerEnabled         bool       `gorm:"not null;default:false" json:"crawler_enabled"`
	CrawlerMode            int        `gorm:"not null;default:1" json:"crawler_mode"`
	CrawlerSourceURL       string     `gorm:"size:500" json:"crawler_source_url,omitempty"`
	CrawlerIntervalMinutes int        `gorm:"not null;default:0" json:"crawler_interval_minutes"`
	CrawlerWeekdays        int        `gorm:"not null;default:0" json:"crawler_weekdays"`
	CrawlerLastChapter     int        `gorm:"not null;default:0" json:"crawler_last_chapter"`
	CrawlerTargetChapter   int        `gorm:"not null;default:0" json:"crawler_target_chapter"`
	CrawlerLastCheckedAt   *time.Time `json:"crawler_last_checked_at,omitempty"`
	CreatedAt              time.Time  `json:"created_at"`
	UpdatedAt              time.Time  `json:"updated_at"`
	Genres                 []Genre    `gorm:"many2many:comic_genres;" json:"genres,omitempty"`
}

type ComicGenre struct {
	ComicID uint `gorm:"primaryKey"`
	GenreID uint `gorm:"primaryKey"`
}

type ComicFollow struct {
	ID             uint       `gorm:"primaryKey" json:"id"`
	UserID         uint       `gorm:"index;not null" json:"user_id"`
	ComicID        uint       `gorm:"index;not null" json:"comic_id"`
	LastNotifiedAt *time.Time `json:"last_notified_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
	Comic          Comic      `gorm:"foreignKey:ComicID" json:"comic,omitempty"`
}

type Chapter struct {
	ID            uint      `gorm:"primaryKey" json:"id"`
	ComicID       uint      `gorm:"index;not null" json:"comic_id"`
	ChapterNumber int       `gorm:"index;not null" json:"chapter_number"`
	Title         string    `gorm:"size:200" json:"title,omitempty"`
	ContentURL    string    `gorm:"size:255" json:"content_url,omitempty"`
	PageCount     int       `gorm:"not null;default:0" json:"page_count"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type ReadingHistory struct {
	ID          uint       `gorm:"primaryKey" json:"id"`
	UserID      uint       `gorm:"index;not null" json:"user_id"`
	ChapterID   uint       `gorm:"index;not null" json:"chapter_id"`
	ReadAt      time.Time  `gorm:"index" json:"read_at"`
	ReadingTime int        `gorm:"not null" json:"reading_time"`
	RewardedAt  *time.Time `json:"rewarded_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

func (ReadingHistory) TableName() string {
	return "reading_history"
}

type PointsLog struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"index;not null" json:"user_id"`
	ChapterID *uint     `gorm:"index" json:"chapter_id,omitempty"`
	Points    int       `gorm:"not null" json:"points"`
	Reason    string    `gorm:"size:50;not null" json:"reason"`
	CreatedAt time.Time `json:"created_at"`
}

func (PointsLog) TableName() string {
	return "points_log"
}

type UserAdblock struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"unique;not null" json:"user_id"`
	Adblock   bool      `gorm:"not null" json:"adblock"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (UserAdblock) TableName() string {
	return "user_adblock"
}

type AdsStat struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	Date        time.Time `gorm:"type:date;unique" json:"date"`
	Impressions int       `gorm:"not null;default:0" json:"impressions"`
	Revenue     float64   `gorm:"not null;default:0" json:"revenue"`
}

type AdsSettings struct {
	ID             uint      `gorm:"primaryKey" json:"id"`
	AdsenseEnabled bool      `gorm:"not null;default:true" json:"adsense_enabled"`
	UpdatedAt      time.Time `json:"updated_at"`
}

func (AdsSettings) TableName() string {
	return "ads_settings"
}

type DirectAd struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	Title       string    `gorm:"size:200;not null" json:"title"`
	ImageURL    string    `gorm:"size:500;not null" json:"image_url"`
	LinkURL     string    `gorm:"size:500;not null" json:"link_url"`
	TrackingKey string    `gorm:"size:100" json:"tracking_key,omitempty"`
	IsActive    bool      `gorm:"not null;default:true" json:"is_active"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type RefreshToken struct {
	ID           uint       `gorm:"primaryKey" json:"id"`
	UserID       uint       `gorm:"index;not null" json:"user_id"`
	TokenHash    string     `gorm:"size:64;unique;not null" json:"-"`
	ExpiresAt    time.Time  `json:"expires_at"`
	RevokedAt    *time.Time `json:"revoked_at,omitempty"`
	ReplacedByID *uint      `gorm:"index" json:"replaced_by_id,omitempty"`
	UserAgent    string     `gorm:"size:255" json:"user_agent,omitempty"`
	IP           string     `gorm:"size:64" json:"ip,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
	LastUsedAt   *time.Time `json:"last_used_at,omitempty"`
}

// SupportTicket – customer support ticket
type SupportTicket struct {
	ID        uint            `gorm:"primaryKey" json:"id"`
	UserID    uint            `gorm:"index;not null" json:"user_id"`
	Subject   string          `gorm:"size:255;not null" json:"subject"`
	Status    string          `gorm:"size:20;not null;default:open" json:"status"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
	Messages  []TicketMessage `gorm:"foreignKey:TicketID" json:"messages,omitempty"`
	User      User            `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

// TicketMessage – individual message within a ticket thread
type TicketMessage struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	TicketID     uint      `gorm:"index;not null" json:"ticket_id"`
	UserID       uint      `gorm:"index;not null" json:"user_id"`
	Body         string    `gorm:"type:text;not null" json:"body"`
	IsAdminReply bool      `gorm:"not null;default:false" json:"is_admin_reply"`
	CreatedAt    time.Time `json:"created_at"`
	User         User      `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

// TrafficLog – daily view count per comic/chapter
type TrafficLog struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	ComicID   uint      `gorm:"index;not null" json:"comic_id"`
	ChapterID *uint     `gorm:"index" json:"chapter_id,omitempty"`
	Date      time.Time `gorm:"type:date;index;column:date" json:"date"`
	ViewCount int64     `gorm:"not null;default:0" json:"view_count"`
}

// RolePermission – DB-driven RBAC permission rows
type RolePermission struct {
	Role       string `gorm:"size:20;not null;primaryKey" json:"role"`
	Permission string `gorm:"size:100;not null;primaryKey" json:"permission"`
}

// ActivityLog – audit trail for admin actions
type ActivityLog struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"index;not null" json:"user_id"`
	Action    string    `gorm:"size:100;not null" json:"action"`
	Detail    string    `gorm:"type:text" json:"detail,omitempty"`
	IP        string    `gorm:"size:64" json:"ip,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// CrawlerLog – log for crawler runs
type CrawlerLog struct {
	ID                uint       `gorm:"primaryKey" json:"id"`
	ComicID           uint       `gorm:"index;not null" json:"comic_id"`
	Mode              int        `gorm:"not null" json:"mode"`
	Trigger           string     `gorm:"size:20;not null" json:"trigger"`
	Status            string     `gorm:"size:20;not null" json:"status"`
	Message           string     `gorm:"type:text" json:"message,omitempty"`
	SourceURL         string     `gorm:"size:500" json:"source_url,omitempty"`
	LastChapterBefore int        `gorm:"not null;default:0" json:"last_chapter_before"`
	LastChapterAfter  int        `gorm:"not null;default:0" json:"last_chapter_after"`
	TargetChapter     int        `gorm:"not null;default:0" json:"target_chapter"`
	StartedAt         time.Time  `json:"started_at"`
	FinishedAt        *time.Time `json:"finished_at,omitempty"`
	DurationMs        int64      `gorm:"not null;default:0" json:"duration_ms"`
	TriggeredBy       *uint      `gorm:"index" json:"triggered_by,omitempty"`
}
