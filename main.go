package main

import (
    "context"
    "encoding/json"
    "fmt"
    "io"
    "log"
    "math"
    "net/http"
    "os"
    "path/filepath"
    //"strconv"
    "time"

    "github.com/gorilla/mux"
    "github.com/joho/godotenv"

    "go.mongodb.org/mongo-driver/bson"
    "go.mongodb.org/mongo-driver/mongo"
    "go.mongodb.org/mongo-driver/mongo/options"
)

const (
    ELO_K_FACTOR = 32.0
)

// ----- Models -----
type League struct {
    ID      string `bson:"_id,omitempty" json:"id"`
    Name    string `bson:"name" json:"name"`
    LogoURL string `bson:"logo_url,omitempty" json:"logo_url,omitempty"`
}

type Team struct {
    ID       string  `bson:"_id,omitempty" json:"id"`
    LeagueID string  `bson:"league_id" json:"league_id"`
    Name     string  `bson:"name" json:"name"`
    ELO      float64 `bson:"elo_rating" json:"elo_rating"`
    LogoURL  string  `bson:"logo_url,omitempty" json:"logo_url,omitempty"`
}

type ScheduleStatus string

const (
    StatusScheduled ScheduleStatus = "scheduled"
    StatusCompleted ScheduleStatus = "completed"
    StatusCanceled  ScheduleStatus = "canceled"
)

type Schedule struct {
    ID         string          `bson:"_id,omitempty" json:"id"`
    LeagueID   string          `bson:"league_id" json:"league_id"`
    HomeTeamID string          `bson:"home_team_id" json:"home_team_id"`
    AwayTeamID string          `bson:"away_team_id" json:"away_team_id"`
    MatchDate  time.Time       `bson:"match_date" json:"match_date"`
    Status     ScheduleStatus  `bson:"status" json:"status"`
    HomeScore  *int            `bson:"home_score,omitempty" json:"home_score,omitempty"`
    AwayScore  *int            `bson:"away_score,omitempty" json:"away_score,omitempty"`
    UpdatedAt  time.Time       `bson:"updated_at,omitempty" json:"updated_at,omitempty"`
}

// ----- Global Variables -----
var (
    mongoClient *mongo.Client
    db          *mongo.Database

    adminUsername string
    adminPassword string
)

// ----- Utility: Load Env -----
func init() {
    err := godotenv.Load()
    if err != nil {
        log.Printf("Warning: .env file not found. Proceeding with system ENV or defaults.")
    }

    adminUsername = os.Getenv("ADMIN_USERNAME")
    adminPassword = os.Getenv("ADMIN_PASSWORD")
}

// ----- Main -----
func main() {
    // Connect to Mongo
    mongoURI := getEnv("MONGO_URI", "mongodb://localhost:27017")
    mongoDBName := getEnv("MONGO_DBNAME", "eloDB")
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()
    var err error
    mongoClient, err = mongo.Connect(ctx, options.Client().ApplyURI(mongoURI))
    if err != nil {
        log.Fatalf("Mongo connection error: %v\n", err)
    }
    if err = mongoClient.Ping(ctx, nil); err != nil {
        log.Fatalf("Mongo ping error: %v\n", err)
    }
    log.Println("Connected to MongoDB successfully")

    db = mongoClient.Database(mongoDBName)

    // Router setup with Gorilla Mux
    r := mux.NewRouter()

    // Public endpoints
    r.HandleFunc("/leagues", GetLeaguesHandler).Methods("GET")
    r.HandleFunc("/teams", GetTeamsHandler).Methods("GET")
    r.HandleFunc("/schedules", GetSchedulesHandler).Methods("GET")

    // Admin endpoints (protected by middleware)
    admin := r.PathPrefix("/admin").Subrouter()
    admin.Use(AdminAuthMiddleware) // apply authentication for admin routes

    admin.HandleFunc("/leagues", CreateLeagueHandler).Methods("POST")
    admin.HandleFunc("/teams", CreateTeamHandler).Methods("POST")
    admin.HandleFunc("/schedules", CreateScheduleHandler).Methods("POST")
    admin.HandleFunc("/schedules/{id}/result", UpdateScheduleResultHandler).Methods("PUT")
    admin.HandleFunc("/teams/{id}/logo", UploadTeamLogoHandler).Methods("POST")

    // Serve static folder for uploaded logos (e.g. /uploads/some_logo.png)
    // You can serve it at /uploads path
    r.PathPrefix("/uploads/").Handler(http.StripPrefix("/uploads/", http.FileServer(http.Dir("./uploads/"))))
    // Serving the website pages
    r.PathPrefix("/").Handler(http.StripPrefix("/", http.FileServer(http.Dir("./web"))))

    // Start server
    port := getEnv("PORT", "8080")
    log.Printf("Server running on :%s\n", port)
    log.Fatal(http.ListenAndServe(":"+port, r))
}

// ----- Env Helper -----
func getEnv(key, fallback string) string {
    val := os.Getenv(key)
    if val == "" {
        return fallback
    }
    return val
}

// ----- Admin Authentication Middleware -----
func AdminAuthMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Simple Basic Auth approach:
        username, password, ok := r.BasicAuth()
        if !ok {
            // Prompt for login
            w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
            http.Error(w, "Unauthorized", http.StatusUnauthorized)
            return
        }
        if username != adminUsername || password != adminPassword {
            http.Error(w, "Forbidden", http.StatusForbidden)
            return
        }

        // If valid, proceed
        next.ServeHTTP(w, r)
    })
}

// ===========================================================================
// ===============================  HANDLERS  ================================
// ===========================================================================

// ---------------------- Public: GET Leagues ----------------------
func GetLeaguesHandler(w http.ResponseWriter, r *http.Request) {
    coll := db.Collection("leagues")
    cursor, err := coll.Find(r.Context(), bson.M{})
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    defer cursor.Close(r.Context())

    var leagues []League
    if err := cursor.All(r.Context(), &leagues); err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    respondJSON(w, leagues, http.StatusOK)
}

// ---------------------- Public: GET Teams ----------------------
func GetTeamsHandler(w http.ResponseWriter, r *http.Request) {
    // Optionally, filter by league=? param
    coll := db.Collection("teams")

    // parse optional ?league_id= param
    leagueID := r.URL.Query().Get("league_id")
    filter := bson.M{}
    if leagueID != "" {
        filter["league_id"] = leagueID
    }

    cursor, err := coll.Find(r.Context(), filter)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    defer cursor.Close(r.Context())

    var teams []Team
    if err := cursor.All(r.Context(), &teams); err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    respondJSON(w, teams, http.StatusOK)
}

// ---------------------- Public: GET Schedules ----------------------
func GetSchedulesHandler(w http.ResponseWriter, r *http.Request) {
    coll := db.Collection("schedules")
    // optionally filter by league_id or status
    leagueID := r.URL.Query().Get("league_id")
    status := r.URL.Query().Get("status")

    filter := bson.M{}
    if leagueID != "" {
        filter["league_id"] = leagueID
    }
    if status != "" {
        filter["status"] = status
    }

    cursor, err := coll.Find(r.Context(), filter)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    defer cursor.Close(r.Context())

    var schedules []Schedule
    if err := cursor.All(r.Context(), &schedules); err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    respondJSON(w, schedules, http.StatusOK)
}

// ---------------------- Admin: CREATE League ----------------------
func CreateLeagueHandler(w http.ResponseWriter, r *http.Request) {
    var league League
    if err := json.NewDecoder(r.Body).Decode(&league); err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }
    coll := db.Collection("leagues")
    _, err := coll.InsertOne(r.Context(), league)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    respondJSON(w, map[string]string{"message": "League created"}, http.StatusCreated)
}

// ---------------------- Admin: CREATE Team ----------------------
func CreateTeamHandler(w http.ResponseWriter, r *http.Request) {
    var team Team
    if err := json.NewDecoder(r.Body).Decode(&team); err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }
    // Set default ELO if not provided
    if team.ELO == 0 {
        team.ELO = 1500
    }
    coll := db.Collection("teams")
    _, err := coll.InsertOne(r.Context(), team)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    respondJSON(w, map[string]string{"message": "Team created"}, http.StatusCreated)
}

// ---------------------- Admin: CREATE Schedule ----------------------
func CreateScheduleHandler(w http.ResponseWriter, r *http.Request) {
    var sch Schedule
    if err := json.NewDecoder(r.Body).Decode(&sch); err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }
    if sch.Status == "" {
        sch.Status = StatusScheduled
    }
    if sch.MatchDate.IsZero() {
        sch.MatchDate = time.Now().Add(24 * time.Hour) // default next day
    }
    coll := db.Collection("schedules")
    _, err := coll.InsertOne(r.Context(), sch)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    respondJSON(w, map[string]string{"message": "Schedule created"}, http.StatusCreated)
}

// ---------------------- Admin: UPDATE Schedule Result ----------------------
type UpdateResultRequest struct {
    HomeScore int `json:"home_score"`
    AwayScore int `json:"away_score"`
}

func UpdateScheduleResultHandler(w http.ResponseWriter, r *http.Request) {
    vars := mux.Vars(r)
    scheduleID := vars["id"]
    if scheduleID == "" {
        http.Error(w, "Missing schedule ID", http.StatusBadRequest)
        return
    }

    var req UpdateResultRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }

    // 1) Get schedule
    coll := db.Collection("schedules")
    var sch Schedule
    err := coll.FindOne(r.Context(), bson.M{"_id": scheduleID}).Decode(&sch)
    if err == mongo.ErrNoDocuments {
        http.Error(w, "Schedule not found", http.StatusNotFound)
        return
    } else if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    if sch.Status == StatusCompleted {
        // Already completed
        respondJSON(w, map[string]string{"message": "Schedule already completed"}, http.StatusOK)
        return
    }

    // 2) Determine winner/loser
    var winnerTeamID, loserTeamID string
    var homeScore, awayScore = req.HomeScore, req.AwayScore
    if homeScore > awayScore {
        winnerTeamID = sch.HomeTeamID
        loserTeamID = sch.AwayTeamID
    } else if awayScore > homeScore {
        winnerTeamID = sch.AwayTeamID
        loserTeamID = sch.HomeTeamID
    } else {
        // It's a tie; for ELO, you might do 0.5 each, or skip
        // We'll skip ties in this example
        http.Error(w, "Tie games not supported in this example", http.StatusBadRequest)
        return
    }

    // 3) Fetch teams
    teamsColl := db.Collection("teams")
    var winnerTeam, loserTeam Team
    err = teamsColl.FindOne(r.Context(), bson.M{"_id": winnerTeamID}).Decode(&winnerTeam)
    if err != nil {
        http.Error(w, "Winner team not found", http.StatusBadRequest)
        return
    }
    err = teamsColl.FindOne(r.Context(), bson.M{"_id": loserTeamID}).Decode(&loserTeam)
    if err != nil {
        http.Error(w, "Loser team not found", http.StatusBadRequest)
        return
    }

    // 4) Update ELO
    winnerNew := calculateNewElo(winnerTeam.ELO, loserTeam.ELO, 1.0, ELO_K_FACTOR)
    loserNew := calculateNewElo(loserTeam.ELO, winnerTeam.ELO, 0.0, ELO_K_FACTOR)

    // 5) Update teams
    _, err = teamsColl.UpdateOne(r.Context(), bson.M{"_id": winnerTeamID}, bson.M{"$set": bson.M{"elo_rating": winnerNew}})
    if err != nil {
        http.Error(w, "Failed to update winner ELO", http.StatusInternalServerError)
        return
    }
    _, err = teamsColl.UpdateOne(r.Context(), bson.M{"_id": loserTeamID}, bson.M{"$set": bson.M{"elo_rating": loserNew}})
    if err != nil {
        http.Error(w, "Failed to update loser ELO", http.StatusInternalServerError)
        return
    }

    // 6) Mark schedule as completed
    now := time.Now()
    update := bson.M{
        "$set": bson.M{
            "status":     StatusCompleted,
            "home_score": homeScore,
            "away_score": awayScore,
            "updated_at": now,
        },
    }
    _, err = coll.UpdateOne(r.Context(), bson.M{"_id": scheduleID}, update)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    respondJSON(w, map[string]string{"message": "Match completed and ELO updated"}, http.StatusOK)
}

// ---------------------- Admin: UPLOAD Team Logo (Local Storage) ----------------------
func UploadTeamLogoHandler(w http.ResponseWriter, r *http.Request) {
    vars := mux.Vars(r)
    teamID := vars["id"]
    if teamID == "" {
        http.Error(w, "Missing team ID", http.StatusBadRequest)
        return
    }

    // Parse multipart form
    err := r.ParseMultipartForm(10 << 20) // ~10MB limit
    if err != nil {
        http.Error(w, "Error parsing form data", http.StatusBadRequest)
        return
    }

    file, handler, err := r.FormFile("logo")
    if err != nil {
        http.Error(w, "Error retrieving the file", http.StatusBadRequest)
        return
    }
    defer file.Close()

    // Create uploads folder if it doesn't exist
    uploadDir := "./uploads"
    if _, err := os.Stat(uploadDir); os.IsNotExist(err) {
        os.Mkdir(uploadDir, 0755)
    }

    // Save file locally
    ext := filepath.Ext(handler.Filename)
    filePath := filepath.Join(uploadDir, fmt.Sprintf("%s%s", teamID, ext))
    dst, err := os.Create(filePath)
    if err != nil {
        http.Error(w, "Error creating file on server", http.StatusInternalServerError)
        return
    }
    defer dst.Close()

    _, err = io.Copy(dst, file)
    if err != nil {
        http.Error(w, "Error saving file", http.StatusInternalServerError)
        return
    }

    // Update the team's document with logo path
    logoURL := "/uploads/" + fmt.Sprintf("%s%s", teamID, ext)
    _, err = db.Collection("teams").UpdateOne(r.Context(), bson.M{"_id": teamID},
        bson.M{"$set": bson.M{"logo_url": logoURL}})
    if err != nil {
        http.Error(w, "Failed to update team logo", http.StatusInternalServerError)
        return
    }

    respondJSON(w, map[string]string{"message": "Logo uploaded successfully", "logo_url": logoURL}, http.StatusOK)
}

// ===========================================================================
// ============================  HELPER FUNCTIONS  ============================
// ===========================================================================

// Basic JSON responder
func respondJSON(w http.ResponseWriter, data interface{}, status int) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(status)
    json.NewEncoder(w).Encode(data)
}

// ELO calculation helpers
func calculateExpectedScore(ratingA, ratingB float64) float64 {
    return 1.0 / (1.0 + math.Pow(10, (ratingB - ratingA)/400.0))
}

func calculateNewElo(ratingA, ratingB float64, scoreA float64, k float64) float64 {
    expA := calculateExpectedScore(ratingA, ratingB)
    return ratingA + k*(scoreA - expA)
}
