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
					{
						Method:  "email",
						Minutes: template.reminderMin + 5,
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

func loadEventIds() (map[string]string, error) {
	file, err := os.ReadFile("event_ids.json")
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]string), nil
		}
		return nil, err
	}

	var data StoredEventIds
	if err := json.Unmarshal(file, &data); err != nil {
		return nil, err
	}
	return data.EventIds, nil
}

func getWorkoutDescription(weekday time.Weekday) string {
	// Map each weekday to its workout description
	workoutDescriptions := map[time.Weekday]string{
		time.Monday: `## Chest and Triceps + Cardio (Intense)
### Warm-up (10-12 minutes):
1. Light jogging or marching in place (2 minutes)
2. Arm circles: 10 forward, 10 backward
3. Shoulder rolls: 10 forward, 10 backward
4. Wall push-ups: 2 sets of 10
5. Tricep stretches: Hold for 15 seconds each arm
6. Cat-cow stretch: 10 repetitions
7. Dynamic chest stretches: 10 arm swings

### Strength Training:
- Barbell Bench Press: 5 sets of 5-7 reps
- Incline Dumbbell Press: 4 sets of 8-10 reps
- Dumbbell Flyes: 3 sets of 10-12 reps
- Cable Flyes: 3 sets of 12-15 reps
- Close-Grip Bench Press: 4 sets of 8-10 reps
- Tricep Pushdowns: 4 sets of 10-12 reps
- Dumbbell Overhead Tricep Extensions: 3 sets of 10-12 reps

### Cardio:
- Total time: 35-45 minutes
  - 5 min warm-up: 4.0-4.8 km/h at 0-1% incline
  - 25-35 min main session: 4.8-6.4 km/h, starting at 5-7% incline, increasing to 12-17% as tolerated
  - 5 min cool-down: 4.0-4.8 km/h at 0-1% incline`,

		time.Tuesday: `## Active Recovery and Mobility (Light)
### Warm-up (5-7 minutes):
1. Light walking or marching in place (2 minutes)
2. Arm swings: 10 forward, 10 across body
3. Hip circles: 10 each direction
4. Leg swings: 10 each leg, forward and side
5. Torso twists: 10 each side

### Light Cardio:
- 20-30 min light walking or cycling

### Mobility and Flexibility:
- Dynamic stretching: 10-15 minutes
- Foam rolling: 10-15 minutes
  - Focus on chest and triceps
  - Address any tight areas from yesterday's workout
- Yoga or light bodyweight exercises: 15-20 minutes`,

		time.Wednesday: `## Back and Biceps + Cardio (Intense)
### Warm-up (10-12 minutes):
1. Light jogging or jumping jacks (2 minutes)
2. Arm circles: 10 forward, 10 backward
3. Shoulder blade squeezes: 2 sets of 10
4. Cat-cow stretch: 10 repetitions
5. Standing alternating toe touches: 10 each side
6. Lat stretches: Hold for 15 seconds each side
7. Wrist rotations and flexions: 10 each direction

### Strength Training:
- Deadlifts: 5 sets of 5-7 reps
- Pull-ups or Weighted Lat Pulldowns: 4 sets of 8-10 reps
- Bent-over Barbell Rows: 4 sets of 8-10 reps
- T-Bar Rows: 3 sets of 10-12 reps
- Face Pulls: 3 sets of 12-15 reps
- Barbell Curls: 4 sets of 8-10 reps
- Incline Dumbbell Curls: 3 sets of 10-12 reps
- Hammer Curls: 3 sets of 10-12 reps

### Cardio:
- Same as Monday's session`,

		time.Thursday: `## Core and Balance (Light)
### Warm-up (5-7 minutes):
1. Light walking or marching in place (2 minutes)
2. Torso twists: 10 each side
3. Standing side bends: 10 each side
4. Hip circles: 10 each direction
5. Leg swings: 10 each leg, forward and side

### Core Exercises:
- Planks: 3 sets of 30-60 seconds
- Russian Twists: 3 sets of 15-20 reps
- Bicycle Crunches: 3 sets of 15-20 reps
- Bird Dogs: 3 sets of 10-12 reps each side

### Balance and Stability:
- Single-leg balance: 3 sets of 30 seconds each leg
- Bosu ball squats: 3 sets of 10-12 reps
- Wall sits: 3 sets of 30-60 seconds

### Recovery Focus:
- Foam rolling: 10-15 minutes
  - Focus on back and biceps
  - Address any tight areas from yesterday's workout`,

		time.Friday: `## Legs + Cardio (Intense)
### Warm-up (10-12 minutes):
1. Light jogging or high knees (2 minutes)
2. Ankle rotations: 10 each direction, each foot
3. Bodyweight squats: 2 sets of 10
4. Walking lunges: 10 steps
5. Standing quadriceps stretch: Hold for 15 seconds each leg
6. Standing calf stretches: Hold for 15 seconds each leg
7. Hip flexor stretches: Hold for 15 seconds each side

### Strength Training:
- Squats: 5 sets of 5-7 reps
- Romanian Deadlifts: 4 sets of 8-10 reps
- Leg Press: 4 sets of 10-12 reps
- Walking Lunges: 3 sets of 24 steps
- Leg Extensions: 3 sets of 12-15 reps
- Leg Curls: 3 sets of 12-15 reps
- Standing Calf Raises: 4 sets of 15-20 reps
- Seated Calf Raises: 3 sets of 15-20 reps

### Cardio:
- Same as Monday's session`,

		time.Saturday: `## Upper Body and Flexibility (Light)
### Warm-up (5-7 minutes):
1. Light walking or arm circles (2 minutes)
2. Shoulder rolls: 10 forward, 10 backward
3. Arm swings: 10 forward, 10 across body
4. Torso twists: 10 each side
5. Wrist rotations and flexions: 10 each direction

### Light Upper Body Exercises:
- Push-ups: 3 sets of 8-12 reps
- Resistance band rows: 3 sets of 12-15 reps
- Dumbbell lateral raises: 3 sets of 10-12 reps
- Tricep dips: 3 sets of 10-12 reps

### Flexibility:
- Static stretching routine: 20-30 minutes
- Foam rolling: 10-15 minutes
  - Focus on legs
  - Address any tight areas from yesterday's workout

### Light Cardio:
- 15-20 min light walking or cycling`,

		time.Sunday: `## Complete Rest Day
Today is your designated rest day. Focus on:
- Good nutrition
- Proper hydration
- Quality sleep
- Light walking if desired
- Gentle stretching if needed
- Mental preparation for next week's training

### Recovery Tips:
- Use this time for meal prep
- Practice stress management
- Review next week's workout plans
- Ensure workout clothes are ready
- Check gym bag supplies
- Set goals for the upcoming week`,
	}

	// Get the description for the given weekday
	if desc, ok := workoutDescriptions[weekday]; ok {
		return desc
	}
	return "No workout scheduled for this day"
}

func getWorkoutEmoji(dayType string) string {
	switch dayType {
	case "Intense":
		return "🏋️‍♂️"
	case "Light":
		return "🧘‍♂️"
	default:
		return "💤"
	}
}

func updateExistingEvent(srv *calendar.Service, eventId string, template EventTemplate, timeZone string) error {
	// Get the existing event first
	existingEvent, err := srv.Events.Get("primary", eventId).Do()
	if err != nil {
		return fmt.Errorf("error getting existing event: %v", err)
	}

	// Calculate the start time based on the existing event's start time
	startTime, err := time.Parse(time.RFC3339, existingEvent.Start.DateTime)
	if err != nil {
		return fmt.Errorf("error parsing existing event time: %v", err)
	}

	// Parse the template start time
	templateTime, err := time.Parse("15:04", template.startTime)
	if err != nil {
		return fmt.Errorf("error parsing template time: %v", err)
	}

	// Update the start time while keeping the original date
	finalStartTime := time.Date(
		startTime.Year(), startTime.Month(), startTime.Day(),
		templateTime.Hour(), templateTime.Minute(), 0, 0,
		startTime.Location(),
	)
	finalEndTime := finalStartTime.Add(template.duration)

	// Update the event
	existingEvent.Summary = template.summary
	existingEvent.Description = template.description
	existingEvent.Start = &calendar.EventDateTime{
		DateTime: finalStartTime.Format(time.RFC3339),
		TimeZone: timeZone,
	}
	existingEvent.End = &calendar.EventDateTime{
		DateTime: finalEndTime.Format(time.RFC3339),
		TimeZone: timeZone,
	}
	existingEvent.ColorId = template.colorId
	existingEvent.Reminders = &calendar.EventReminders{
		Overrides: []*calendar.EventReminder{
			{
				Method:  "popup",
				Minutes: template.reminderMin,
			},
			{
				Method:  "email",
				Minutes: template.reminderMin + 5,
			},
		},
		UseDefault:      false,
		ForceSendFields: []string{"UseDefault"},
	}

	_, err = srv.Events.Update("primary", eventId, existingEvent).Do()
	return err
}

func getIntenseWorkout() EventTemplate {
	return EventTemplate{
		summary: fmt.Sprintf("Workout %s", getWorkoutEmoji("Intense")),
		description: "Intense Workout Days:\n\n" +
			"Monday: " + getWorkoutDescription(time.Monday) + "\n\n" +
			"Wednesday: " + getWorkoutDescription(time.Wednesday) + "\n\n" +
			"Friday: " + getWorkoutDescription(time.Friday),
		startTime:   "06:00",
		duration:    90 * time.Minute,
		daysOfWeek:  []time.Weekday{time.Monday, time.Wednesday, time.Friday},
		reminderMin: 30,
		colorId:     "10", // Green
	}
}

func getLightWorkout() EventTemplate {
	return EventTemplate{
		summary: fmt.Sprintf("Recovery Session %s", getWorkoutEmoji("Light")),
		description: "Light Workout Days:\n\n" +
			"Tuesday: " + getWorkoutDescription(time.Tuesday) + "\n\n" +
			"Thursday: " + getWorkoutDescription(time.Thursday) + "\n\n" +
			"Saturday: " + getWorkoutDescription(time.Saturday),
		startTime:   "06:00",
		duration:    60 * time.Minute,
		daysOfWeek:  []time.Weekday{time.Tuesday, time.Thursday, time.Saturday},
		reminderMin: 30,
		colorId:     "7", // Light Green
	}
}

func getRestDay() EventTemplate {
	return EventTemplate{
		summary:     fmt.Sprintf("Rest Day %s", getWorkoutEmoji("Rest")),
		description: getWorkoutDescription(time.Sunday),
		startTime:   "06:00",
		duration:    30 * time.Minute,
		daysOfWeek:  []time.Weekday{time.Sunday},
		reminderMin: 30,
		colorId:     "5", // Yellow
	}
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
	intenseDays := []time.Weekday{time.Monday, time.Wednesday, time.Friday}
	lightDays := []time.Weekday{time.Tuesday, time.Thursday, time.Saturday}

	// Define event templates with colors
	templates := []EventTemplate{
		{
			summary: "Work Hours 💻",
			description: "Remote work time (Berlin office hours)\n\n" +
				"Tasks:\n" +
				"- Check and respond to emails\n" +
				"- Attend scheduled meetings\n" +
				"- Complete assigned tasks\n" +
				"- Document progress\n" +
				"- Coordinate with team members",
			startTime:   "11:00",
			duration:    7 * time.Hour,
			daysOfWeek:  workdays,
			reminderMin: 15,
			colorId:     "1", // Lavender
		},
		{
			summary: "Family Time 🏡",
			description: "Dedicated family time - no work or exercise\n\n" +
				"Guidelines:\n" +
				"- No work-related activities\n" +
				"- Engage in family conversations\n" +
				"- Share meals together\n" +
				"- Participate in family activities\n" +
				"- Be present and mindful",
			startTime:   "18:30",
			duration:    3*time.Hour + 30*time.Minute,
			daysOfWeek:  weekdays,
			reminderMin: 15,
			colorId:     "3", // Grape
		},
		getIntenseWorkout(),
		getLightWorkout(),
		getRestDay(),
		{
			summary: "Pre-workout Meal 🥪",
			description: "GERD-Friendly Pre-workout Nutrition\n\n" +
				"Choose ONE option:\n\n" +
				"Low-Intensity Days (150-180 cal):\n" +
				"1. Oatmeal with banana\n" +
				"   - 1/2 cup plain oats\n" +
				"   - 1 small banana\n" +
				"   - Cinnamon (no sugar)\n\n" +
				"2. Toast with egg white\n" +
				"   - 1 slice whole grain toast\n" +
				"   - 2 egg whites\n\n" +
				"High-Intensity Days (200-250 cal):\n" +
				"1. Protein smoothie\n" +
				"   - 1 scoop vanilla protein\n" +
				"   - 1 banana\n" +
				"   - Almond milk\n\n" +
				"Guidelines:\n" +
				"- Eat 60-90 min before workout\n" +
				"- Small portions only\n" +
				"- Stay upright after eating\n" +
				"- Sip water, don't gulp",
			startTime:   "05:00",
			duration:    15 * time.Minute,
			daysOfWeek:  append(intenseDays, lightDays...),
			reminderMin: 10,
			colorId:     "5",
		},
		{
			summary: "Breakfast 🍳",
			description: "GERD-Friendly Weight Loss Breakfast\n\n" +
				"Choose ONE option (400-450 calories):\n\n" +
				"1. Protein Oatmeal Bowl (400 cal):\n" +
				"   - 1 cup cooked plain oats\n" +
				"   - 1 scoop vanilla protein powder\n" +
				"   - 1 tbsp chia seeds\n" +
				"   - 1/2 banana\n" +
				"   - Cinnamon to taste\n\n" +
				"2. Egg White Breakfast (420 cal):\n" +
				"   - 4 egg whites\n" +
				"   - 1 whole egg\n" +
				"   - 1 slice whole grain toast\n" +
				"   - 1/4 avocado\n" +
				"   - Steamed spinach\n\n" +
				"GERD Tips:\n" +
				"- Eat slowly, chew well\n" +
				"- Stay upright 30 min after\n" +
				"- Avoid coffee initially\n" +
				"- Use pillows to elevate if needed",
			startTime:   "08:00",
			duration:    30 * time.Minute,
			daysOfWeek:  weekdays,
			reminderMin: 10,
			colorId:     "5",
		},
		{
			summary: "Lunch 🥗",
			description: "GERD-Friendly Weight Loss Lunch\n\n" +
				"Choose ONE option (500-550 calories):\n\n" +
				"1. Lean Protein Bowl (520 cal):\n" +
				"   - 5 oz grilled chicken breast\n" +
				"   - 3/4 cup brown rice\n" +
				"   - 1 cup steamed vegetables\n" +
				"   - 1 tbsp olive oil\n" +
				"   - Herbs for seasoning\n\n" +
				"2. Fish & Quinoa (540 cal):\n" +
				"   - 5 oz baked white fish\n" +
				"   - 2/3 cup quinoa\n" +
				"   - 1 cup roasted vegetables\n" +
				"   - 1 tbsp pine nuts\n\n" +
				"GERD Guidelines:\n" +
				"- No raw onions/garlic\n" +
				"- Avoid tomatoes\n" +
				"- Use herbs instead of spices\n" +
				"- Steam or bake, don't fry",
			startTime:   "12:00",
			duration:    45 * time.Minute,
			daysOfWeek:  weekdays,
			reminderMin: 15,
			colorId:     "5",
		},
		{
			summary: "Afternoon Snack 🍎",
			description: "GERD-Friendly Weight Loss Snack\n\n" +
				"Choose ONE option (150-200 calories):\n\n" +
				"1. Protein Snack (160 cal):\n" +
				"   - 1 oz almonds\n" +
				"   - 1 small pear\n\n" +
				"2. Yogurt Mix (180 cal):\n" +
				"   - 3/4 cup Greek yogurt\n" +
				"   - 1/2 tbsp honey\n" +
				"   - 1/4 cup blueberries\n\n" +
				"GERD Tips:\n" +
				"- Small portions only\n" +
				"- Avoid citrus fruits\n" +
				"- No chocolate/mint\n" +
				"- Stay upright",
			startTime:   "15:30",
			duration:    15 * time.Minute,
			daysOfWeek:  weekdays,
			reminderMin: 10,
			colorId:     "5",
		},
		{
			summary: "Dinner 🍲",
			description: "GERD-Friendly Weight Loss Dinner\n\n" +
				"Choose ONE option (400-450 calories):\n\n" +
				"1. Lean Protein Plate (420 cal):\n" +
				"   - 4 oz baked chicken breast\n" +
				"   - 2/3 cup quinoa\n" +
				"   - 1.5 cups steamed vegetables\n" +
				"   - 1 tsp olive oil\n" +
				"   - Herbs for seasoning\n\n" +
				"2. Fish Dinner (440 cal):\n" +
				"   - 5 oz poached white fish\n" +
				"   - 1/2 cup sweet potato\n" +
				"   - 1 cup green beans\n" +
				"   - 1 tsp butter\n\n" +
				"GERD Guidelines:\n" +
				"- Last meal of day\n" +
				"- No sauce/spices\n" +
				"- Steam or bake, don't fry\n" +
				"- Finish 3 hours before bed",
			startTime:   "19:00",
			duration:    30 * time.Minute,
			daysOfWeek:  weekdays,
			reminderMin: 15,
			colorId:     "5",
		},
		{
			summary: "No More Food Today 🚫",
			description: "GERD Management - Stop Eating\n\n" +
				"Important Reminders:\n" +
				"- No more food until tomorrow\n" +
				"- Only water if needed\n" +
				"- No late-night snacks\n" +
				"- Stay upright for at least 3 hours\n" +
				"- Consider a short walk to aid digestion",
			startTime:   "19:30",
			duration:    1 * time.Minute,
			daysOfWeek:  weekdays,
			reminderMin: 5,
			colorId:     "11", // Red
		},
		{
			summary: "Prepare for Sleep 🛌",
			description: "GERD-Safe Sleep Preparation\n\n" +
				"Checklist:\n" +
				"1. Bed Preparation:\n" +
				"   - Elevate head of bed 6-8 inches\n" +
				"   - Use bed risers or wedge pillow\n" +
				"2. Position:\n" +
				"   - Sleep on left side when possible\n" +
				"   - Avoid lying flat\n" +
				"3. Clothing:\n" +
				"   - Wear loose-fitting clothes\n" +
				"   - Avoid tight waistbands\n\n" +
				"Remember:\n" +
				"- No food for at least 3 hours\n" +
				"- Limited water intake\n" +
				"- Practice relaxation techniques",
			startTime:   "22:00",
			duration:    1 * time.Minute,
			daysOfWeek:  weekdays,
			reminderMin: 15,
			colorId:     "9", // Blue
		},
		{
			summary: "Post-Meal Walk 🚶‍♂️",
			description: "Light Exercise After Meal\n\n" +
				"Guidelines:\n" +
				"- Duration: 15-20 minutes\n" +
				"- Pace: Comfortable walking speed\n" +
				"- Breathing: Should be able to hold conversation\n\n" +
				"Benefits:\n" +
				"- Aids digestion\n" +
				"- Reduces GERD symptoms\n" +
				"- Improves blood sugar control\n" +
				"- Light cardio activity",
			startTime:   "13:00",
			duration:    20 * time.Minute,
			daysOfWeek:  lightDays[:2], // Only Tuesday and Thursday
			reminderMin: 10,
			colorId:     "10", // Green
		},
		{
			summary: "Evening Stretching 🧘‍♂️",
			description: "Evening Stretching Routine\n\n" +
				"Sequence (hold each for 15-30 seconds):\n\n" +
				"1. Upper Body:\n" +
				"   - Neck rotations\n" +
				"   - Shoulder rolls\n" +
				"   - Chest stretches\n" +
				"   - Upper back stretches\n\n" +
				"2. Core:\n" +
				"   - Cat-cow stretch\n" +
				"   - Gentle twists\n" +
				"   - Child's pose\n\n" +
				"3. Lower Body:\n" +
				"   - Hamstring stretches\n" +
				"   - Quad stretches\n" +
				"   - Calf stretches\n\n" +
				"Tips:\n" +
				"- Breathe deeply\n" +
				"- Move slowly\n" +
				"- No bouncing\n" +
				"- Stop if pain occurs",
			startTime:   "21:30",
			duration:    15 * time.Minute,
			daysOfWeek:  weekdays,
			reminderMin: 10,
			colorId:     "10", // Green
		},
	}

	// Load existing event IDs
	existingEventIds, err := loadEventIds()
	if err != nil {
		log.Printf("Error loading existing event IDs: %v\n", err)
		existingEventIds = make(map[string]string)
	}

	// Create/Update events and store their IDs
	fmt.Println("\nCreating/Updating events...")
	eventIds := make(map[string]string)

	for _, template := range templates {
		if existingId, exists := existingEventIds[template.summary]; exists {
			// Try to update existing event
			fmt.Printf("Updating existing event: %s\n", template.summary)
			err := updateExistingEvent(srv, existingId, template, timeZone)
			if err != nil {
				fmt.Printf("Error updating event '%s', will create new: %v\n", template.summary, err)
				// If update fails (e.g., event was deleted), create new event
				eventId, err := createRecurringEvent(srv, template, timeZone)
				if err != nil {
					fmt.Printf("Error creating event '%s': %v\n", template.summary, err)
					continue
				}
				fmt.Printf("Successfully created new event: %s\n", template.summary)
				eventIds[template.summary] = eventId
			} else {
				fmt.Printf("Successfully updated event: %s\n", template.summary)
				eventIds[template.summary] = existingId
			}
		} else {
			// Create new event
			fmt.Printf("Creating new event: %s\n", template.summary)
			eventId, err := createRecurringEvent(srv, template, timeZone)
			if err != nil {
				fmt.Printf("Error creating event '%s': %v\n", template.summary, err)
				continue
			}
			fmt.Printf("Successfully created event: %s\n", template.summary)
			eventIds[template.summary] = eventId
		}
	}

	// Save the event IDs
	if err := saveEventIds(eventIds); err != nil {
		log.Printf("Error saving event IDs: %v\n", err)
	}
}
