package app

import (
	"errors"
	"fmt"
	"franklyner/gores/middleware"
	"time"
)

var (
	ErrorMultipleUsers = errors.New("multiple users found")
	ErrNotFound        = errors.New("record not found")
)

type User struct {
	Name     string
	Email    string
	Phone    string
	Password string
}

type Entry struct {
	ID          int
	User        string
	Begin       time.Time
	End         time.Time
	Bemerkungen string
}

type Day struct {
	DayOfMonth int
	Month      int
	Entry      Entry
}

type Calendar struct {
	PrevYear  int
	Year      int
	NextYear  int
	PrevMonth int
	Month     int
	NextMonth int
	MonthYear string
	Weeks     [][]Day
}

func LoadUser(username string) (User, error) {
	rows, err := middleware.DB.Query("SELECT name, email, phone, pwd FROM users WHERE name=?", username)
	if err != nil {
		return User{}, fmt.Errorf("error fetching user (%s): %w", username, err)
	}
	defer rows.Close()

	var name, email, phone, password string

	if !rows.Next() {
		if rows.Err() != nil {
			return User{}, fmt.Errorf("error fetching user: %w", rows.Err())
		}
		return User{}, fmt.Errorf("no user found (%s): %w", username, ErrNotFound)
	}
	err = rows.Scan(&name, &email, &phone, &password)
	if err != nil {
		return User{}, fmt.Errorf("error fetching rows from db: %w", err)
	}

	return User{
		Name:     name,
		Email:    email,
		Phone:    phone,
		Password: password,
	}, nil
}

func LoadCalendarForMonth(year, month int) (Calendar, error) {
	start := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.Local)
	firstDay := start
	weekday := int(start.Weekday())
	start = start.AddDate(0, 0, -1*weekday)
	end := start.AddDate(0, 0, 35) // always fetching 5 full weeks

	entries, err := loadEntries(start, end)
	if err != nil {
		return Calendar{}, fmt.Errorf("error loading entries (%d, %d): %w", year, month, err)
	}

	currDay := start
	weeks := make([][]Day, 0, 5)
	for i := 0; i < 5; i++ {
		weeks = append(weeks, make([]Day, 0, 7))
	}
	entryIdx := 0
	for i := 0; i < 35; i++ {
		d := Day{
			DayOfMonth: currDay.Day(),
			Month:      month,
		}
		entry := entries[entryIdx]
		if currDay.After(entry.Begin.AddDate(0, 0, -1)) && currDay.Before(entry.End.AddDate(0, 0, 1)) {
			d.Entry = entry
		}
		currDay = currDay.AddDate(0, 0, 1)

		if currDay.After(entry.End) {
			entryIdx++
		}
		weekIdx := i / 7
		weeks[weekIdx] = append(weeks[weekIdx], d)
	}
	return Calendar{
		PrevYear:  firstDay.AddDate(-1, 0, 0).Year(),
		Year:      year,
		NextYear:  firstDay.AddDate(1, 0, 0).Year(),
		PrevMonth: int(firstDay.AddDate(0, -1, 0).Month()),
		Month:     month,
		NextMonth: int(firstDay.AddDate(0, 1, 0).Month()),
		MonthYear: fmt.Sprintf("%s %d", time.Month(month).String(), year),
		Weeks:     weeks,
	}, nil
}

func loadEntries(start, end time.Time) ([]Entry, error) {
	entries := make([]Entry, 0, 35)
	dbres, err := middleware.DB.Query("select res_id, user, begin, end, bemerkungen from entries where end >= ? and begin <= ?", start, end)
	if err != nil {
		return nil, fmt.Errorf("error fetching query: %w", err)
	}
	if dbres.Err() != nil {
		return nil, fmt.Errorf("query returned error: %w", err)
	}

	defer dbres.Close()

	for dbres.Next() {
		var entry Entry
		err = dbres.Scan(&entry.ID, &entry.User, &entry.Begin, &entry.End, &entry.Bemerkungen)
		if err != nil {
			return nil, fmt.Errorf("error scanning row: %w", err)
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

func daysIn(m time.Month, year int) int {
	return time.Date(year, m+1, 0, 0, 0, 0, 0, time.UTC).Day()
}
