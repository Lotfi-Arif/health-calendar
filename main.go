package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	calendar "google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
)

// Event template structure
type EventTemplate struct {
	summary     string
	description string
	startTime   string
	duration    time.Duration
	daysOfWeek  []time.Weekday
	reminderMin int64
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

func getTokenFromWeb(config *oauth2.Config) *oauth2.Token {
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Printf("Go to the following link in your browser then type the "+
		"authorization code: \n%v\n", authURL)

	var authCode string
	if _, err := fmt.Scan(&authCode); err != nil {
		log.Fatalf("Unable to read authorization code: %v", err)
	}

	tok, err := config.Exchange(context.TODO(), authCode)
	if err != nil {
		log.Fatalf("Unable to retrieve token from web: %v", err)
	}
	return tok
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

func saveToken(path string, token *oauth2.Token) {
	fmt.Printf("Saving credential file to: %s\n", path)
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		log.Fatalf("Unable to cache oauth token: %v", err)
	}
	defer f.Close()
	json.NewEncoder(f).Encode(token)
}

func main() {
	ctx := context.Background()

	// Read the credentials file
	b, err := os.ReadFile("credentials.json")
	if err != nil {
		log.Fatalf("Unable to read client secret file: %v", err)
	}

	// Configure OAuth2
	config, err := google.ConfigFromJSON(b, calendar.CalendarScope)
	if err != nil {
		log.Fatalf("Unable to parse client secret file to config: %v", err)
	}
	client := getClient(config)

	// Create Calendar service
	srv, err := calendar.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		log.Fatalf("Unable to retrieve Calendar client: %v", err)
	}

	// First, list available calendars
	calendars, err := srv.CalendarList.List().Do()
	if err != nil {
		log.Fatalf("Unable to retrieve calendar list: %v", err)
	}

	fmt.Println("\nAvailable calendars:")
	for _, c := range calendars.Items {
		fmt.Printf("- %s\n", c.Summary)
	}

	// Create a test event for tomorrow
	startTime := time.Now().Add(24 * time.Hour)
	endTime := startTime.Add(1 * time.Hour)

	event := &calendar.Event{
		Summary:     "Test Health Calendar Event",
		Description: "This is a test event to verify calendar integration",
		Start: &calendar.EventDateTime{
			DateTime: startTime.Format(time.RFC3339),
			TimeZone: "Asia/Jakarta",
		},
		End: &calendar.EventDateTime{
			DateTime: endTime.Format(time.RFC3339),
			TimeZone: "Asia/Jakarta",
		},
		Reminders: &calendar.EventReminders{
			UseDefault: true, // Use calendar's default reminders
		},
	}

	fmt.Println("\nCreating test event...")
	event, err = srv.Events.Insert("primary", event).Do()
	if err != nil {
		log.Fatalf("Unable to create event: %v", err)
	}
	fmt.Printf("Event created successfully! View it here: %s\n", event.HtmlLink)

	// Try to create another event with custom reminders
	startTime = startTime.Add(2 * time.Hour)
	endTime = endTime.Add(2 * time.Hour)

	eventWithReminder := &calendar.Event{
		Summary:     "Test Event with Custom Reminder",
		Description: "This is a test event with a custom reminder",
		Start: &calendar.EventDateTime{
			DateTime: startTime.Format(time.RFC3339),
			TimeZone: "Asia/Jakarta",
		},
		End: &calendar.EventDateTime{
			DateTime: endTime.Format(time.RFC3339),
			TimeZone: "Asia/Jakarta",
		},
		Reminders: &calendar.EventReminders{
			UseDefault: false,
			Overrides: []*calendar.EventReminder{
				{
					Method:  "popup",
					Minutes: 30,
				},
			},
		},
	}

	fmt.Println("\nCreating test event with custom reminder...")
	eventWithReminder, err = srv.Events.Insert("primary", eventWithReminder).Do()
	if err != nil {
		fmt.Printf("Note: Could not create event with custom reminder: %v\n", err)
	} else {
		fmt.Printf("Event with custom reminder created successfully! View it here: %s\n", eventWithReminder.HtmlLink)
	}
}
