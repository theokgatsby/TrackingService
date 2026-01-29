package main

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type Session struct {
	gorm.Model
	Title    string  `json:"title"`
	Category string  `json:"category"`
	Rate     float64 `json:"rate"`
}

type SessionResponse struct {
	ID        uint       `json:"id"`
	CreatedAt time.Time  `json:"created_at"`
	DeletedAt *time.Time `json:"deleted_at,omitempty"`
	Title     string     `json:"title"`
	Category  string     `json:"category"`
	Payment   float64    `json:"payment"`
	Duration  string     `json:"duration,omitempty"`
}

var db *gorm.DB

func main() {
	var err error
	db, err = gorm.Open(sqlite.Open("sessions.db"), &gorm.Config{})
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}

	db.AutoMigrate(&Session{})

	router := gin.Default()

	// CORS
	router.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	})

	router.GET("/", serveDashboard)
	router.GET("/sessions", getSessions)
	router.POST("/sessions", createSession)
	router.PATCH("/sessions/:id/stop", stopSession)

	log.Println("Server starting on :8080")
	router.Run(":8080")
}

func getSessions(c *gin.Context) {
	var sessions []Session
	db.Unscoped().Find(&sessions)

	response := make([]SessionResponse, len(sessions))
	for i, s := range sessions {
		response[i] = toResponse(s)
	}

	c.JSON(http.StatusOK, response)
}

func createSession(c *gin.Context) {
	var session Session
	if err := c.BindJSON(&session); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	db.Create(&session)
	fmt.Printf("Created session: %s\n", session.Title)

	c.JSON(http.StatusCreated, toResponse(session))
}

func stopSession(c *gin.Context) {
	id := c.Param("id")
	var session Session

	if err := db.Unscoped().First(&session, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "session not found"})
		return
	}

	if session.DeletedAt.Valid {
		c.JSON(http.StatusOK, toResponse(session))
		return
	}

	db.Model(&session).Update("deleted_at", time.Now())
	db.Unscoped().First(&session, id)

	fmt.Printf("Stopped session: %s\n", session.Title)
	c.JSON(http.StatusOK, toResponse(session))
}

func toResponse(s Session) SessionResponse {
	var payment float64
	var duration string
	var deletedAt *time.Time

	if s.DeletedAt.Valid {
		elapsed := s.DeletedAt.Time.Sub(s.CreatedAt)
		hours := int(elapsed.Hours()) + 1
		payment = float64(hours) * s.Rate
		duration = fmt.Sprintf("%.0fs", elapsed.Seconds())
		deletedAt = &s.DeletedAt.Time
	} else {
		elapsed := time.Since(s.CreatedAt)
		hours := int(elapsed.Hours()) + 1
		payment = float64(hours) * s.Rate
	}

	return SessionResponse{
		ID:        s.ID,
		CreatedAt: s.CreatedAt,
		DeletedAt: deletedAt,
		Title:     s.Title,
		Category:  s.Category,
		Payment:   payment,
		Duration:  duration,
	}
}

func serveDashboard(c *gin.Context) {
	c.Header("Content-Type", "text/html")
	c.File("index.html")
}