// store_test.go
package main

import (
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestUpdateUserSession(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	store := &Store{DB: db}

	us := &UserSession{
		UserID:       123,
		ChatID:       456,
		LanguageCode: "en",
		Longitude:    10.0,
		Latitude:     20.0,
	}

	mock.ExpectExec("REPLACE INTO user_session").WithArgs(us.UserID, us.ChatID, us.LanguageCode, us.Longitude, us.Latitude, sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(1, 1))

	err = store.UpdateUserSession(us)
	if err != nil {
		t.Errorf("error was not expected while updating user session: %s", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}

func TestGetSessionByChatID(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	store := &Store{DB: db}

	rows := sqlmock.NewRows([]string{"chatid", "userid", "language", "longitude", "latitude", "created_at"}).
		AddRow(456, 123, "en", 10.0, 20.0, time.Now())

	mock.ExpectQuery("SELECT (.+) FROM user_session WHERE chatid=?").WithArgs(456).WillReturnRows(rows)

	us, err := store.GetSessionByChatID(456)
	if err != nil {
		t.Errorf("error was not expected while getting user session: %s", err)
	}

	if us.ChatID != 456 || us.UserID != 123 || us.LanguageCode != "en" || us.Longitude != 10.0 || us.Latitude != 20.0 {
		t.Errorf("returned user session does not match expected values")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}

func TestAddAQISubscription(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	store := &Store{DB: db}

	chatID := int64(456)

	mock.ExpectQuery("SELECT (.+) FROM user_session WHERE chatid=?").WithArgs(chatID).
		WillReturnRows(sqlmock.NewRows([]string{"chatid", "userid", "language", "longitude", "latitude", "created_at"}).
			AddRow(chatID, 123, "en", 10.0, 20.0, time.Now()))

	mock.ExpectQuery("SELECT .+ FROM subscription WHERE chat_id=\\? AND enabled=1").
		WithArgs(chatID).
		WillReturnRows(sqlmock.NewRows([]string{"chat_id", "language", "longitude", "latitude", "aqi", "created_at"}))

	mock.ExpectQuery("SELECT data FROM data_point WHERE chat_id=\\? ORDER BY created_at DESC LIMIT 1").WithArgs(chatID).
		WillReturnRows(sqlmock.NewRows([]string{"data"}).AddRow(`{"main":{"aqi":2}}`))

	mock.ExpectExec("INSERT INTO subscription").WithArgs(chatID, "en", 10.0, 20.0, 2, 1, sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err = store.AddAQISubscription(chatID)
	if err != nil {
		t.Errorf("error was not expected while adding AQI subscription: %s", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}

func TestDeleteAQISubscriptions(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	store := &Store{DB: db}

	chatID := int64(456)

	mock.ExpectExec("UPDATE subscription SET enabled=0 WHERE chat_id=?").WithArgs(chatID).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = store.DeleteAQISubscriptions(chatID)
	if err != nil {
		t.Errorf("error was not expected while deleting AQI subscriptions: %s", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}
