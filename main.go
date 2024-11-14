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
	colorId     string
}

// StoredEventIds structure to save event IDs
type StoredEventIds struct {
	EventIds map[string]string `json:"event_ids"` // map[summary]eventId
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

func createRecurringEvent(srv *calendar.Service, template EventTemplate, timeZone string) (string, error) {
	// Calculate the start of next week
	now := time.Now()
	daysUntilMonday := (8 - int(now.Weekday())) % 7
	startDate := now.AddDate(0, 0, daysUntilMonday)

	var lastEventId string

	for _, day := range template.daysOfWeek {
		dayOffset := (int(day) - int(startDate.Weekday()) + 7) % 7
		eventStart := startDate.AddDate(0, 0, dayOffset)

		startTimeComponents := template.startTime
		eventStartTime, err := time.Parse("15:04", startTimeComponents)
		if err != nil {
			return "", fmt.Errorf("error parsing time: %v", err)
		}

		finalStartTime := time.Date(
			eventStart.Year(), eventStart.Month(), eventStart.Day(),
			eventStartTime.Hour(), eventStartTime.Minute(), 0, 0,
			eventStart.Location(),
		)
		finalEndTime := finalStartTime.Add(template.duration)

		event := &calendar.Event{
			Summary:     template.summary,
			Description: template.description,
			Start: &calendar.EventDateTime{
				DateTime: finalStartTime.Format(time.RFC3339),
				TimeZone: timeZone,
			},
			End: &calendar.EventDateTime{
				DateTime: finalEndTime.Format(time.RFC3339),
				TimeZone: timeZone,
			},
			Recurrence: []string{"RRULE:FREQ=WEEKLY"},
			ColorId:    template.colorId,
			Reminders: &calendar.EventReminders{
				Overrides: []*calendar.EventReminder{
					{
						Method:  "popup",
						Minutes: template.reminderMin,
					},
				},
				UseDefault:      false,
				ForceSendFields: []string{"UseDefault"},
			},
		}

		createdEvent, err := srv.Events.Insert("primary", event).Do()
		if err != nil {
			return "", fmt.Errorf("unable to create event: %v", err)
		}
		lastEventId = createdEvent.Id
	}

	return lastEventId, nil
}

func saveEventIds(eventIds map[string]string) error {
	data := StoredEventIds{
		EventIds: eventIds,
	}
	file, err := json.MarshalIndent(data, "", "    ")
	if err != nil {
		return fmt.Errorf("error marshaling event IDs: %v", err)
	}
	return os.WriteFile("event_ids.json", file, 0644)
}

func main() {
	ctx := context.Background()
	b, err := os.ReadFile("credentials.json")
	if err != nil {
		log.Fatalf("Unable to read client secret file: %v", err)
	}

	config, err := google.ConfigFromJSON(b, calendar.CalendarScope)
	if err != nil {
		log.Fatalf("Unable to parse client secret file to config: %v", err)
	}
	client := getClient(config)

	srv, err := calendar.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		log.Fatalf("Unable to retrieve Calendar client: %v", err)
	}

	timeZone := "Asia/Jakarta"
	workdays := []time.Weekday{
		time.Monday, time.Tuesday, time.Wednesday, time.Thursday, time.Friday,
	}
	weekdays := []time.Weekday{
		time.Sunday, time.Monday, time.Tuesday, time.Wednesday, time.Thursday, time.Friday, time.Saturday,
	}

	// Simplified event templates
	templates := []EventTemplate{
		{
			summary:     "Work Hours 💻",
			description: "Remote work time (Berlin office hours)",
			startTime:   "10:00",
			duration:    7 * time.Hour,
			daysOfWeek:  workdays,
			reminderMin: 15,
			colorId:     "1", // Lavender
		},
		{
			summary:     "Gym or At-Home Workout 🏋️‍♂️ (Check Obsidian)",
			description: "Strength training or cardio session",
			startTime:   "7:00",
			duration:    45 * time.Minute,
			daysOfWeek:  []time.Weekday{time.Monday, time.Wednesday, time.Friday},
			reminderMin: 15,
			colorId:     "10", // Green
		},
		{
			summary:     "Light Exercise 🧘‍♂️ (Check Description)",
			description: "Light movement and stretching session",
			startTime:   "7:00",
			duration:    30 * time.Minute,
			daysOfWeek:  []time.Weekday{time.Tuesday, time.Thursday},
			reminderMin: 15,
			colorId:     "7", // Light Green
		},
		{
			summary:     "Saturday Grocery Shopping 🛒 (Check Description)",
			description: "Weekly grocery shopping for meal prep",
			startTime:   "10:00",
			duration:    2 * time.Hour,
			daysOfWeek:  []time.Weekday{time.Saturday},
			reminderMin: 30,
			colorId:     "5", // Yellow
		},
		{
			summary:     "Sunday Meal Prep ⚡️ (Check Description)",
			description: "Weekly meal preparation session",
			startTime:   "14:00",
			duration:    3 * time.Hour,
			daysOfWeek:  []time.Weekday{time.Sunday},
			reminderMin: 30,
			colorId:     "5", // Yellow
		},
		{
			summary:     "Time to Head Home 🏠",
			description: "Evening wrap-up reminder",
			startTime:   "22:00",
			duration:    15 * time.Minute,
			daysOfWeek:  weekdays,
			reminderMin: 10,
			colorId:     "11", // Red
		},
		{
			summary:     "Breakfast 🍳",
			description: "GERD-friendly breakfast",
			startTime:   "08:00",
			duration:    30 * time.Minute,
			daysOfWeek:  weekdays,
			reminderMin: 10,
			colorId:     "5", // Yellow
		},
		{
			summary:     "Lunch 🥗",
			description: "GERD-friendly lunch",
			startTime:   "12:00",
			duration:    45 * time.Minute,
			daysOfWeek:  weekdays,
			reminderMin: 15,
			colorId:     "5", // Yellow
		},
		{
			summary:     "Afternoon Snack 🍎",
			description: "GERD-friendly snack",
			startTime:   "15:30",
			duration:    15 * time.Minute,
			daysOfWeek:  weekdays,
			reminderMin: 10,
			colorId:     "5", // Yellow
		},
		{
			summary:     "Dinner 🍲",
			description: "GERD-friendly dinner",
			startTime:   "19:00",
			duration:    30 * time.Minute,
			daysOfWeek:  weekdays,
			reminderMin: 15,
			colorId:     "5", // Yellow
		},
		{
			summary:     "Evening Stretching 🧘‍♂️",
			description: "Evening flexibility routine",
			startTime:   "21:30",
			duration:    15 * time.Minute,
			daysOfWeek:  weekdays,
			reminderMin: 10,
			colorId:     "10", // Green
		},
	}

	// Create events and store their IDs
	fmt.Println("\nCreating events...")
	eventIds := make(map[string]string)

	for _, template := range templates {
		// Create new event
		fmt.Printf("Creating new event: %s\n", template.summary)
		eventId, err := createRecurringEvent(srv, template, timeZone)
		if err != nil {
			fmt.Printf("Error creating event '%s': %v\n", template.summary, err)
			continue
		}
		eventIds[template.summary] = eventId
	}

	// Save the event IDs
	if err := saveEventIds(eventIds); err != nil {
		log.Printf("Error saving event IDs: %v\n", err)
	}
}
