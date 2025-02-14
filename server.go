package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/calendar/v3"
)

type Meeting struct {
	Title     string   `json:"title"`
	StartTime string   `json:"startTime"`
	EndTime   string   `json:"endTime"`
	Attendees []string `json:"attendees"`
}

func getClient(config *oauth2.Config) *http.Client {
	tokFile := "token.json"
	tok, err := tokenFromFile(tokFile)
	if err != nil {
		tok = getTokenFromWeb(config)
		saveToken(tokFile, tok)
	}
	return config.Client(context.Background(), tok)
}

func tokenFromFile(file string) (*oauth2.Token, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	tok := &oauth2.Token{}
	err = json.NewDecoder(f).Decode(tok)
	return tok, err
}

func getTokenFromWeb(config *oauth2.Config) *oauth2.Token {
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Printf("Go to the following link in your browser then type the authorization code: \n%v\n", authURL)

	var authCode string
	if _, err := fmt.Scan(&authCode); err != nil {
		log.Fatalf("Unable to read authorization code: %v", err)
	}

	tok, err := config.Exchange(context.Background(), authCode)
	if err != nil {
		log.Fatalf("Unable to retrieve token from web: %v", err)
	}
	return tok
}

func saveToken(file string, token *oauth2.Token) {
	f, err := os.Create(file)
	if err != nil {
		log.Fatalf("Unable to create token file: %v", err)
	}
	defer f.Close()
	json.NewEncoder(f).Encode(token)
}

func subtractTimeFromUTC(utcTime string, hours int, minutes int) string {
	parsedTime, err := time.Parse(time.RFC3339, utcTime)
	if err != nil {
		fmt.Println("Error parsing UTC time:", err)
		return utcTime // Return the original if there's an error
	}

	// Subtract hours and minutes
	updatedTime := parsedTime.Add(-time.Duration(hours)*time.Hour - time.Duration(minutes)*time.Minute)

	return updatedTime.Format(time.RFC3339) // Return adjusted time in RFC3339 format
}

func createEvent(c *gin.Context) {
	var meeting Meeting
	if err := c.BindJSON(&meeting); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input"})
		return
	}

	b, err := os.ReadFile("credentials.json")
	if err != nil {
		log.Fatalf("Unable to read credentials.json: %v", err)
	}

	config, err := google.ConfigFromJSON(b, calendar.CalendarScope)
	if err != nil {
		log.Fatalf("Unable to parse credentials: %v", err)
	}

	client := getClient(config)
	srv, err := calendar.New(client)
	if err != nil {
		log.Fatalf("Unable to retrieve Calendar client: %v", err)
	}

	event := &calendar.Event{
		Summary: meeting.Title,
		Start: &calendar.EventDateTime{
			DateTime: subtractTimeFromUTC(meeting.StartTime, 5, 30),
			TimeZone: "UTC"},
		End: &calendar.EventDateTime{
			DateTime: subtractTimeFromUTC(meeting.EndTime, 5, 30),
			TimeZone: "UTC"},
		Attendees: []*calendar.EventAttendee{},
	}
	fmt.Println(event.Start.DateTime, event.End.DateTime)

	for _, email := range meeting.Attendees {
		event.Attendees = append(event.Attendees, &calendar.EventAttendee{Email: email})
	}

	event, err = srv.Events.Insert("primary", event).Do()
	if err != nil {
		log.Printf("Error creating event: %v", err) // 🔍 Logs the real error
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Event created", "link": event.HtmlLink})
}

func main() {
	router := gin.Default()

	// 🔥 Enable CORS to allow frontend requests
	router.Use(cors.New(cors.Config{
		AllowOrigins: []string{"*"},
		AllowMethods: []string{"GET", "POST", "OPTIONS"},
		AllowHeaders: []string{"Content-Type"},
	}))

	router.POST("/api/create-meeting", createEvent)
	router.Run(":5000")
}
