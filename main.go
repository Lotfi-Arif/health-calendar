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

func createRecurringEvent(srv *calendar.Service, template EventTemplate, timeZone string) error {
	// Calculate the start of next week
	now := time.Now()
	daysUntilMonday := (8 - int(now.Weekday())) % 7
	startDate := now.AddDate(0, 0, daysUntilMonday)

	for _, day := range template.daysOfWeek {
		// Calculate the day offset
		dayOffset := (int(day) - int(startDate.Weekday()) + 7) % 7
		eventStart := startDate.AddDate(0, 0, dayOffset)

		// Parse the time
		startTimeComponents := template.startTime
		eventStartTime, err := time.Parse("15:04", startTimeComponents)
		if err != nil {
			return fmt.Errorf("error parsing time: %v", err)
		}

		// Combine date and time
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
			Reminders: &calendar.EventReminders{
				Overrides: []*calendar.EventReminder{
					{
						Method:  "popup",
						Minutes: template.reminderMin,
					},
					{
						Method:  "email",
						Minutes: template.reminderMin + 5, // Email reminder slightly earlier
					},
				},
				UseDefault:      false,
				ForceSendFields: []string{"UseDefault"},
			},
		}

		_, err = srv.Events.Insert("primary", event).Do()
		if err != nil {
			return fmt.Errorf("unable to create event: %v", err)
		}
	}
	return nil
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
	weekdays := []time.Weekday{
		time.Monday, time.Tuesday, time.Wednesday, time.Thursday, time.Friday,
	}

	// Define event templates
	templates := []EventTemplate{
		// Work Schedule
		{
			summary:     "Work Hours 💻",
			description: "Remote work time (Berlin office hours)",
			startTime:   "11:00",
			duration:    7 * time.Hour,
			daysOfWeek:  weekdays,
			reminderMin: 15,
		},
		// Family Time
		{
			summary:     "Family Time 🏡",
			description: "Dedicated family time - no work or exercise",
			startTime:   "18:30",
			duration:    3*time.Hour + 30*time.Minute,
			daysOfWeek:  weekdays,
			reminderMin: 15,
		},
		// Gym Days (Mon/Wed/Fri)
		{
			summary:     "Gym Workout 🏋️‍♂️",
			description: "Gym session - focus on consistency over intensity",
			startTime:   "06:00",
			duration:    90 * time.Minute,
			daysOfWeek:  []time.Weekday{time.Monday, time.Wednesday, time.Friday},
			reminderMin: 30,
		},
		// Meals
		{
			summary:     "Pre-workout Snack 🍌",
			description: "Light snack before gym (gym days only)",
			startTime:   "05:30",
			duration:    15 * time.Minute,
			daysOfWeek:  []time.Weekday{time.Monday, time.Wednesday, time.Friday},
			reminderMin: 10,
		},
		{
			summary:     "Breakfast 🍳",
			description: "Protein-rich breakfast",
			startTime:   "08:00",
			duration:    30 * time.Minute,
			daysOfWeek:  weekdays,
			reminderMin: 10,
		},
		{
			summary:     "Lunch 🥗",
			description: "Healthy lunch before work",
			startTime:   "12:00",
			duration:    45 * time.Minute,
			daysOfWeek:  weekdays,
			reminderMin: 15,
		},
		{
			summary:     "Afternoon Snack 🍎",
			description: "Healthy afternoon snack",
			startTime:   "15:30",
			duration:    15 * time.Minute,
			daysOfWeek:  weekdays,
			reminderMin: 10,
		},
		{
			summary:     "Dinner 🍲",
			description: "Last meal of the day - keep it light for GERD management",
			startTime:   "19:00",
			duration:    30 * time.Minute,
			daysOfWeek:  weekdays,
			reminderMin: 15,
		},
		// GERD Management Reminders
		{
			summary:     "No More Food Today 🚫",
			description: "Stop eating for GERD management - no food after this point",
			startTime:   "19:30",
			duration:    1 * time.Minute,
			daysOfWeek:  weekdays,
			reminderMin: 5,
		},
		{
			summary:     "Prepare for Sleep 🛌",
			description: "Elevate head of bed, avoid lying flat for GERD management",
			startTime:   "22:00",
			duration:    1 * time.Minute,
			daysOfWeek:  weekdays,
			reminderMin: 15,
		},
		// Exercise Reminders for Non-Gym Days
		{
			summary:     "Post-Meal Walk 🚶‍♂️",
			description: "15-20 minute walk after meal (non-gym days)",
			startTime:   "13:00",
			duration:    20 * time.Minute,
			daysOfWeek:  []time.Weekday{time.Tuesday, time.Thursday},
			reminderMin: 10,
		},
		{
			summary:     "Evening Stretching 🧘‍♂️",
			description: "Basic stretching routine before bed",
			startTime:   "21:30",
			duration:    15 * time.Minute,
			daysOfWeek:  weekdays,
			reminderMin: 10,
		},
	}

	// Create all events
	for _, template := range templates {
		fmt.Printf("Creating recurring event: %s\n", template.summary)
		err := createRecurringEvent(srv, template, timeZone)
		if err != nil {
			fmt.Printf("Error creating event '%s': %v\n", template.summary, err)
		} else {
			fmt.Printf("Successfully created recurring event: %s\n", template.summary)
		}
	}
}
