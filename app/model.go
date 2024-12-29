package app

import (
	"errors"
	"fmt"
	"franklyner/gores/middleware"
	"log"
	"strings"
	"time"
)

const (
	ClassnameRightMonth         = "rightmonth"
	ClassnameWrongMonth         = "wrongmonth"
	ClassenamRightMonthEntry    = "res_rightmonth"
	ClassenamWrongMonthEntry    = "res_wrongmonth"
	ClassnameRightMonthOwnEntry = "eig_res_rightmonth"
	ClassnameWrongMonthOwnEntry = "eig_res_wrongmonth"
)

var (
	ErrorMultipleUsers = errors.New("multiple users found")
	ErrNotFound        = errors.New("record not found")
	ErrConflict        = errors.New("conflict")

	GermanMonths = map[int]string{
		1:  "Januar",
		2:  "Februar",
		3:  "März",
		4:  "April",
		5:  "Mai",
		6:  "Juni",
		7:  "Juli",
		8:  "August",
		9:  "September",
		10: "Oktober",
		11: "November",
		12: "Dezember",
	}
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
	IsOwn       bool
	Month       int
	Year        int
}

type Day struct {
	DayOfMonth int
	Month      int
	Entry      Entry
	Classname  string
}

type Calendar struct {
	PrevYear       int
	Year           int
	NextYear       int
	PrevMonth      int
	PrevMonthName  string
	Month          int
	NextMonth      int
	NextMonthName  string
	MonthYear      string
	DecMonth       int
	DecYear        int
	IncMonth       int
	IncYear        int
	Weeks          [][]Day
	Message        string
	AllDaysInMonth []int
	AllMonths      []int
	AllYears       []int
	AllEntries     []Entry
}

func CreateEntry(entry Entry) error {
	query := `
		SELECT * FROM entries
		WHERE (
			BEGIN <= ?
			AND END >= ?
		)
		OR (
			BEGIN <= ?
			AND END >= ?
		)
		OR (
			BEGIN >= ?
			AND END <= ?
		)`
	rows, err := middleware.DB.Query(query, entry.Begin, entry.Begin, entry.End, entry.End, entry.Begin, entry.End)
	if err != nil {
		return fmt.Errorf("error querying for conflicts: %w", err)
	}
	if rows.Next() {
		return ErrConflict
	}

	_, err = middleware.DB.Exec("insert into entries (user, begin, end, bemerkungen) values (?,?,?,?)", entry.User, entry.Begin, entry.End, entry.Bemerkungen)
	if err != nil {
		return fmt.Errorf("error inserting entry into db: %w", err)
	}

	return nil
}

func DeleteEntry(id int, user string) error {
	rows, err := middleware.DB.Query("select user from entries where res_id=?", id)
	if err != nil {
		return fmt.Errorf("error checking for entry for delete")
	}
	var eu string
	if !rows.Next() {
		log.Default().Printf("entry with id %d was not found", id)
		return nil
	}
	err = rows.Scan(&eu)
	if err != nil {
		return fmt.Errorf("error scanning user of entry to delete: %w", err)
	}
	if strings.EqualFold(user, eu) || strings.EqualFold("frank", eu) {
		_, err := middleware.DB.Exec("delete from entries where res_id = ?", id)
		if err != nil {
			return fmt.Errorf("error deleteing entry (%d): %w", id, err)
		}
	}
	return nil
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
	start := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	firstDay := start
	weekday := int(start.Weekday())
	start = start.AddDate(0, 0, -1*weekday)
	end := start.AddDate(0, 0, 35) // always fetching 5 full weeks

	entries, err := loadEntries(start, end)
	if err != nil {
		return Calendar{}, fmt.Errorf("error loading entries (%d, %d): %w", year, month, err)
	}
	log.Default().Printf("found %d entries", len(entries))

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
		hasEntry := false
		isOwn := false
		if len(entries) > entryIdx {
			entry := entries[entryIdx]
			if (currDay.Equal(entry.Begin) || currDay.After(entry.Begin)) && (currDay.Equal(entry.End) || currDay.Before(entry.End)) {
				d.Entry = entry
				hasEntry = true
				isOwn = entry.IsOwn
			}
			if currDay.Equal(entry.End) {
				entryIdx++
			}
		}

		d.Classname = getClassname(month, currDay, hasEntry, isOwn)

		currDay = currDay.AddDate(0, 0, 1)

		weekIdx := i / 7
		weeks[weekIdx] = append(weeks[weekIdx], d)
	}
	return Calendar{
		PrevYear:       firstDay.AddDate(-1, 0, 0).Year(),
		Year:           year,
		NextYear:       firstDay.AddDate(1, 0, 0).Year(),
		PrevMonth:      int(firstDay.AddDate(0, -1, 0).Month()),
		PrevMonthName:  GermanMonths[int(firstDay.AddDate(0, -1, 0).Month())],
		IncMonth:       int(firstDay.AddDate(0, 1, 0).Month()),
		IncYear:        int(firstDay.AddDate(0, 1, 0).Year()),
		DecMonth:       int(firstDay.AddDate(0, -1, 0).Month()),
		DecYear:        int(firstDay.AddDate(0, -1, 0).Year()),
		Month:          month,
		NextMonth:      int(firstDay.AddDate(0, 1, 0).Month()),
		NextMonthName:  GermanMonths[int(firstDay.AddDate(0, 1, 0).Month())],
		MonthYear:      fmt.Sprintf("%s %d", GermanMonths[month], year),
		Weeks:          weeks,
		AllDaysInMonth: getAllDaysInMonth(),
		AllMonths:      []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12},
		AllYears:       getAllYears(year),
		AllEntries:     entries,
	}, nil
}

func getClassname(month int, date time.Time, hasEntry bool, isOwn bool) string {
	isRightMonth := date.Month() == time.Month(month)

	switch {
	case isRightMonth && hasEntry && isOwn:
		return ClassnameRightMonthOwnEntry
	case !isRightMonth && hasEntry && isOwn:
		return ClassnameWrongMonthOwnEntry
	case isRightMonth && hasEntry && !isOwn:
		return ClassenamRightMonthEntry
	case !isRightMonth && hasEntry && !isOwn:
		return ClassenamWrongMonthEntry
	case isRightMonth && !hasEntry:
		return ClassnameRightMonth
	case !isRightMonth && !hasEntry:
		return ClassnameWrongMonth
	}
	return ""
}

func getAllYears(year int) []int {
	years := make([]int, 0, 4)
	for i := 0; i < 4; i++ {
		years = append(years, year)
		year = year + 1
	}
	return years
}

func getAllDaysInMonth() []int {
	days := []int{}
	for day := 1; day <= 31; day++ {
		days = append(days, day)
	}
	return days
}

func loadEntries(start, end time.Time) ([]Entry, error) {
	user := middleware.Session.Get("username")
	entries := make([]Entry, 0, 35)
	dbres, err := middleware.DB.Query("select res_id, user, begin, end, bemerkungen from entries where end >= ? and begin <= ? order by begin asc", start, end)
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
		if entry.User == user {
			entry.IsOwn = true
		}
		entry.Year = start.Year()
		entry.Month = int(start.Month()) + 1
		entries = append(entries, entry)
	}
	return entries, nil
}

func daysIn(m time.Month, year int) int {
	return time.Date(year, m+1, 0, 0, 0, 0, 0, time.UTC).Day()
}
