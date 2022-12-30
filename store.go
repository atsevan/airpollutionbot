package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

var sqlSchema = `
CREATE TABLE IF NOT EXISTS "user_session" (
	"chatid" INTEGER PRIMARY KEY,
	"userid" INTEGER,
	"language" VARCHAR(64) NULL,
	"longitude" REAL NULL,
	"latitude" REAL NULL,
	"created_at" DATE
);

CREATE TABLE IF NOT EXISTS "data_point" (
	"id" INTEGER PRIMARY KEY AUTOINCREMENT,
	"chat_id" INTEGER,
	"data" JSON,
	"created_at" DATE,
	FOREIGN KEY("chat_id") REFERENCES user_session("chatid")
);

CREATE TABLE IF NOT EXISTS "subscription" (
	"id" INTEGER PRIMARY KEY AUTOINCREMENT,
	"chat_id" INTEGER,
	"language" VARCHAR(64) NULL,
	"longitude" REAL,
	"latitude" REAL,
	"aqi" INT,
	"enabled" INTEGER,
	"created_at" DATE
); 
`

// ErrNotificationExists is returted on attempt to add an existing location
var ErrNotificationExists = errors.New("location is already subscribed")

type UserSession struct {
	UserID       int64
	ChatID       int64
	LanguageCode string
	Longitude    float64
	Latitude     float64
	CreatedAt    time.Time
}

// SetLocation adds a location to the UserSession
func (us *UserSession) SetLocation(l *Location) {
	us.Latitude = l.Latitude
	us.Longitude = l.Longitude
}

// Store keeps an UserSessions, DataPoints and Subscriptions
type Store struct {
	DB        *sql.DB
	CacheTime time.Duration
}

func (s *Store) Init() error {
	_, err := s.DB.Exec(sqlSchema)
	if err != nil {
		return err
	}
	return nil
}

// UpdateUserSession replaces the UserSession in a DB
func (s *Store) UpdateUserSession(n *UserSession) error {
	_, err := s.DB.Exec("REPLACE INTO user_session (userid, chatid, language, longitude, latitude, created_at) VALUES (?, ?, ?, ?, ?, ?)",
		n.UserID, n.ChatID, n.LanguageCode, n.Longitude, n.Latitude, time.Now())
	if err != nil {
		return fmt.Errorf("UpdateUserSession: %v", err)
	}
	return nil
}

// AddDataPoint adds a DataPoint for the ChatID into DB for caching purposes. Returns an error or nil
func (s *Store) AddDataPoint(chatID int64, dps *[]DataPoint) error {
	for _, dp := range *dps {
		dataPoint, err := json.Marshal(dp)
		if err != nil {
			return fmt.Errorf("marshaling DP: %v ", err)
		}
		_, err = s.DB.Exec("INSERT into `data_point` (`chat_id`, `data`, `created_at`) VALUES(?, ?, ?)", chatID, dataPoint, time.Unix(dp.Dt, 0))
		if err != nil {
			return fmt.Errorf("updating DB: %v", err)
		}
	}
	return nil
}

// GetSessionByChatID returns an UserSession by ChatID. Or error
func (s *Store) GetSessionByChatID(chatID int64) (*UserSession, error) {
	var us UserSession
	err := s.DB.QueryRow("SELECT chatid, userid, language, longitude, latitude, created_at FROM user_session WHERE chatid=?", chatID).Scan(
		&us.ChatID,
		&us.UserID,
		&us.LanguageCode,
		&us.Longitude,
		&us.Latitude,
		&us.CreatedAt,
	)
	if err != nil {
		log.Print("GetSessionByChatID: ", err)
		return &UserSession{}, err
	}
	return &us, nil
}

// GetLastPD returns latest DataPoint for the ChatID
func (s *Store) GetLastPD(chatID int64) (*DataPoint, error) {

	var dp DataPoint
	var data []byte
	err := s.DB.QueryRow("SELECT data FROM data_point WHERE chat_id=? ORDER BY created_at DESC LIMIT 1", chatID).Scan(&data)
	if err != nil {
		if err == sql.ErrNoRows {
			return &DataPoint{}, nil
		}
		return &DataPoint{}, err
	}
	json.Unmarshal(data, &dp)

	return &dp, nil
}

// AQISubscription represents a Users subscription to AQI updates
type AQISubscription struct {
	ID int64
	UserSession
	AirQualityIndex
}

// AddNotification gathers the latest data for the chatID and create a new AQISubscription record
func (s *Store) AddAQISubscription(chatID int64) error {
	us, err := s.GetSessionByChatID(chatID)
	if err != nil {
		return err
	}

	// duplicate check
	subs, err := s.ListAQISubscriptions(chatID)
	if err != nil {
		return err
	}
	for _, s := range *subs {
		if math.Abs(s.Latitude-us.Latitude) < 0.0009 || math.Abs(s.Longitude-us.Longitude) < 0.0009 {
			log.Print("notificaiton already exists")
			return ErrNotificationExists
		}
	}

	dp, err := s.GetLastPD(chatID)
	if err != nil {
		return err
	}
	_, err = s.DB.Exec("INSERT INTO subscription (chat_id, language, longitude, latitude, aqi, enabled, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)",
		us.ChatID, us.LanguageCode, us.Longitude, us.Latitude, dp.GetAQI(), 1, time.Now())
	if err != nil {
		return fmt.Errorf("addAQISubscription: %v", err)
	}
	return nil
}

// ListAQISubscriptions returns AQISubscriptions for the chatID. And error on DB errors
func (s *Store) ListAQISubscriptions(chatID int64) (*[]AQISubscription, error) {
	var uss []AQISubscription
	rows, err := s.DB.Query("SELECT chat_id, language, longitude, latitude, aqi, created_at FROM subscription WHERE chat_id=? AND enabled=1", chatID)
	if err != nil {
		return &[]AQISubscription{}, err
	}

	for rows.Next() {
		subs := AQISubscription{}

		err := rows.Scan(&subs.ChatID, &subs.LanguageCode, &subs.Longitude, &subs.Latitude, &subs.AirQualityIndex, &subs.CreatedAt)
		if err != nil {
			return &[]AQISubscription{}, err
		}
		uss = append(uss, subs)
	}

	return &uss, nil
}

// DeleteAQISubscriptions disabled all AQISubscriptions for the chatID
func (s *Store) DeleteAQISubscriptions(chatID int64) error {
	_, err := s.DB.Exec("UPDATE subscription SET enabled=0 WHERE chat_id=?", chatID)
	if err != nil {
		return err
	}
	return nil
}

// ListEnabledSubscriptions returns all active AQISubscriptions
func (s *Store) ListEnabledSubscriptions() (*[]AQISubscription, error) {
	var subs []AQISubscription
	rows, err := s.DB.Query("SELECT id, chat_id, language, longitude, latitude, aqi, created_at FROM subscription WHERE enabled=1")
	if err != nil {
		return &[]AQISubscription{}, err
	}
	for rows.Next() {
		sub := AQISubscription{}

		err := rows.Scan(&sub.ID, &sub.ChatID, &sub.LanguageCode, &sub.Longitude, &sub.Latitude, &sub.AirQualityIndex, &sub.CreatedAt)
		if err != nil {
			return &[]AQISubscription{}, err
		}
		subs = append(subs, sub)
	}

	return &subs, nil
}

// UpdateSubscriptionAQI sets the AirQualityIndex for a subcription. Returns an error on DB error
func (s *Store) UpdateSubscriptionAQI(subID int64, aqi AirQualityIndex) error {
	_, err := s.DB.Exec("UPDATE subscription SET aqi=? WHERE id=?", aqi, subID)
	if err != nil {
		return err
	}
	return nil
}

// ClenupAQISubscriptions cleans up disabled AQISubscriptions. Returns an error on DB error
func (s *Store) ClenupAQISubscriptions() error {
	_, err := s.DB.Exec("DELETE subscription WHERE enabled=0")
	if err != nil {
		return err
	}
	return nil
}

// ClenupDataPoint deletes DataPoints older than 12 hours
func (s *Store) ClenupDataPoint() error {
	_, err := s.DB.Exec("DELETE data_point WHERE created_at <= datetime('now', '-12 hour')")
	if err != nil {
		return err
	}
	return nil
}
