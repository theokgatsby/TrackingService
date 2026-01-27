package main

import (
	"fmt"
	"log"
	"math"
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
	UpdatedAt time.Time  `json:"updated_at"`
	DeletedAt *time.Time `json:"deleted_at,omitempty"`
	Title     string     `json:"title"`
	Category  string     `json:"category"`
	Payment   float64    `json:"payment"`
	Duration  string     `json:"duration"`
}

var db *gorm.DB

func main() {
	if err := initDB(); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}

	router := setupRouter()
	
	log.Println("Starting server on :8080")
	if err := router.Run(":8080"); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

func initDB() error {
	var err error
	db, err = gorm.Open(sqlite.Open("sessions.db"), &gorm.Config{})
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}

	if err := db.AutoMigrate(&Session{}); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	return nil
}

func setupRouter() *gin.Engine {
	router := gin.Default()

	// Add CORS middleware
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

	router.GET("/sessions", getSessions)
	router.GET("/sessions/:id", getSessionByID)
	router.POST("/sessions", postSessions)
	router.PATCH("/sessions/:id/stop", stopSession)

	return router
}

func toResponse(s Session) SessionResponse {
	var end *time.Time
	if s.DeletedAt.Valid {
		end = &s.DeletedAt.Time
	}

	payment := calculatePayment(s.Rate, s.CreatedAt, end)

	resp := SessionResponse{
		ID:        s.ID,
		CreatedAt: s.CreatedAt,
		UpdatedAt: s.UpdatedAt,
		Title:     s.Title,
		Category:  s.Category,
		Payment:   payment,
	}

	if end != nil {
		duration := end.Sub(s.CreatedAt)
		resp.DeletedAt = end
		resp.Duration = fmt.Sprintf("%.0fs", duration.Seconds())
	}

	return resp
}

func getSessions(c *gin.Context) {
	var sessions []Session
	if err := db.Unscoped().Find(&sessions).Error; err != nil {
		c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve sessions"})
		return
	}

	response := make([]SessionResponse, len(sessions))
	for i, s := range sessions {
		response[i] = toResponse(s)
	}

	c.IndentedJSON(http.StatusOK, response)
}

func postSessions(c *gin.Context) {
	var newSession Session
	if err := c.ShouldBindJSON(&newSession); err != nil {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := db.Create(&newSession).Error; err != nil {
		c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": "failed to create session"})
		return
	}

	c.IndentedJSON(http.StatusCreated, toResponse(newSession))
}

func getSessionByID(c *gin.Context) {
	id := c.Param("id")
	var s Session

	if err := db.Unscoped().First(&s, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.IndentedJSON(http.StatusNotFound, gin.H{"error": "session not found"})
		} else {
			c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve session"})
		}
		return
	}

	c.IndentedJSON(http.StatusOK, toResponse(s))
}

func stopSession(c *gin.Context) {
	id := c.Param("id")
	var s Session

	if err := db.Unscoped().First(&s, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.IndentedJSON(http.StatusNotFound, gin.H{"error": "session not found"})
		} else {
			c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve session"})
		}
		return
	}

	if s.DeletedAt.Valid {
		c.IndentedJSON(http.StatusOK, toResponse(s))
		return
	}

	now := time.Now()
	if err := db.Model(&s).Update("deleted_at", now).Error; err != nil {
		c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": "failed to stop session"})
		return
	}

	if err := db.Unscoped().First(&s, id).Error; err != nil {
		c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve updated session"})
		return
	}

	c.IndentedJSON(http.StatusOK, toResponse(s))
}

func calculatePayment(rate float64, start time.Time, end *time.Time) float64 {
	var elapsed time.Duration

	if end != nil {
		elapsed = end.Sub(start)
	} else {
		elapsed = time.Since(start)
	}

	hours := elapsed.Hours()
	billableHours := int(math.Ceil(hours))

	return (float64(billableHours) + 1) * rate
}
