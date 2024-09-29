package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/joho/godotenv"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatalf("Error loading .env file")
	}

	apiKey := os.Getenv("GOOGLE_MAPS_API_KEY")
	if apiKey == "" {
		log.Fatal("GOOGLE_MAPS_API)")
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
        tmpl := template.Must(template.ParseFiles("index.html"))
        data := struct {
            GoogleMapsAPIKey string
        }{
            GoogleMapsAPIKey: apiKey,
        }
        tmpl.Execute(w, data)
    })

	fmt.Println("a", apiKey)
	
	app := fiber.New()

	app.Use(cors.New())
	

	// Serve the HTML form (for frontend)
	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendFile("./index.html")
	})
    app.Get("/google-maps-api", func(c *fiber.Ctx) error {
        apiKey := os.Getenv("GOOGLE_MAPS_API_KEY")
        script := fmt.Sprintf(`
            var script = document.createElement('script');
            script.src = "https://maps.googleapis.com/maps/api/js?key=%s&callback=initMap&libraries=places";
            script.async = true;
            script.defer = true;
            document.head.appendChild(script);
        `, apiKey)
        c.Set("Content-Type", "application/javascript")
        return c.SendString(script)
    })
	// Route to calculate biking and driving time, emissions, and calories
	app.Post("/calculate", calculateRoute)

	log.Fatal(app.Listen(":3000"))
}

// Function to calculate routes and return time, emissions, and calories
func calculateRoute(c *fiber.Ctx) error {
	// Get user inputs from the request body
	origin := strings.TrimSpace(c.FormValue("origin"))
	destination := strings.TrimSpace(c.FormValue("destination"))

	// Log the incoming data
	log.Printf("Received data - Origin: %s, Destination: %s", origin, destination)

	if origin == "" || destination == "" {
		return c.Status(400).SendString("Please provide both origin and destination")
	}

	// Fetch travel times for biking and driving
	bikingTime, bikingDistance, err := getTravelDetails("bicycling", origin, destination)
	if err != nil {
		log.Printf("Error fetching biking details: %v", err)
		return c.Status(500).SendString("Error fetching biking details")
	}

	drivingTime, drivingDistance, err := getTravelDetails("driving", origin, destination)
	if err != nil {
		log.Printf("Error fetching driving details: %v", err)
		return c.Status(500).SendString("Error fetching driving details")
	}

	// Calculate emissions and calories
	emissions := calculateEmissions(drivingDistance)
	calories := calculateCalories(bikingDistance)

	// Log the data being returned
	log.Printf("Results - Biking Time: %s, Driving Time: %s, Emissions: %f kg CO₂, Calories Burned: %f",
		bikingTime, drivingTime, emissions, calories)

	// Send the results back as JSON
	return c.JSON(fiber.Map{
		"bikingTime":    bikingTime,
		"drivingTime":   drivingTime,
		"emissions":     emissions,
		"caloriesBurned": calories,
	})
}


// Function to get travel time and distance using Google Maps API
func getTravelDetails(mode, origin, destination string) (string, float64, error) {
	apiKey := os.Getenv("GOOGLE_MAPS_API_KEY")
	url := fmt.Sprintf("https://maps.googleapis.com/maps/api/directions/json?origin=%s&destination=%s&mode=%s&key=%s",
		url.QueryEscape(origin), url.QueryEscape(destination), mode, apiKey)

	res, err := http.Get(url)
	if err != nil {
		return "", 0, fmt.Errorf("failed to make request to Google Maps API: %w", err)
	}
	defer res.Body.Close()

	// Check for non-2xx status code
	if res.StatusCode != http.StatusOK {
		return "", 0, fmt.Errorf("received non-OK HTTP status from Google Maps API: %d", res.StatusCode)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		return "", 0, fmt.Errorf("failed to decode Google Maps API response: %w", err)
	}

	// Check if Google Maps API returned a valid result
	if status, ok := result["status"].(string); !ok || status != "OK" {
		return "", 0, fmt.Errorf("Google Maps API error: %v", result["status"])
	}

	// Parse routes, legs, duration, and distance
	if routes, ok := result["routes"].([]interface{}); ok && len(routes) > 0 {
		if legs, ok := routes[0].(map[string]interface{})["legs"].([]interface{}); ok && len(legs) > 0 {
			leg := legs[0].(map[string]interface{})

			// Get duration
			duration, durationOk := leg["duration"].(map[string]interface{})["text"].(string)
			// Get distance (in meters)
			distanceMeters, distanceOk := leg["distance"].(map[string]interface{})["value"].(float64)

			if durationOk && distanceOk {
				// Convert distance to kilometers
				return duration, distanceMeters / 1000, nil
			}
		}
	}

	return "", 0, fmt.Errorf("no valid route found in Google Maps API response")
}

// Function to calculate emissions based on driving distance (in km)
func calculateEmissions(distance float64) float64 {
	// Example: 0.21 kg CO2 per km for a car
	const emissionRatePerKm = 0.21
	return distance * emissionRatePerKm
}

// Function to calculate calories burned based on biking distance (in km)
func calculateCalories(distance float64) float64 {
	// Example: 50 calories per km biking
	const caloriesPerKm = 50
	return distance * caloriesPerKm
}
