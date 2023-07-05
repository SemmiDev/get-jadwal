package main

import (
	"database/sql"
	"fmt"
	"net/mail"
	"os"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/goccy/go-json"
	"github.com/gofiber/fiber/v2"
)

func main() {
	mysqlHost := getEnv("MYSQL_HOST", "localhost")
	mysqlPort := getEnv("MYSQL_PORT", "3306")
	mysqlUser := getEnv("MYSQL_USER", "root")
	mysqlPassword := getEnv("MYSQL_PASSWORD", "")
	mysqlDbName := getEnv("MYSQL_DBNAME", "get_jadwal")

	dbConfig := mysql.Config{
		User:                 mysqlUser,
		Passwd:               mysqlPassword,
		Net:                  "tcp",
		Addr:                 fmt.Sprintf("%s:%s", mysqlHost, mysqlPort),
		DBName:               mysqlDbName,
		AllowNativePasswords: true,
		Params:               map[string]string{"charset": "utf8mb4"},
	}

	db, err := sql.Open("mysql", dbConfig.FormatDSN())
	if err != nil {
		panic(err.Error())
	}
	defer db.Close()

	db.SetConnMaxLifetime(5 * time.Hour)
	db.SetMaxIdleConns(25)
	db.SetMaxOpenConns(100)

	err = db.Ping()
	if err != nil {
		panic(err.Error())
	}

	db.Exec(`CREATE TABLE IF NOT EXISTS users (
		id INT AUTO_INCREMENT PRIMARY KEY,
		email VARCHAR(100) NOT NULL);`)
	db.Exec(`CREATE TABLE IF NOT EXISTS schedules (
		id INT AUTO_INCREMENT PRIMARY KEY,
		user_id INT,
		title VARCHAR(50) NOT NULL,
		day VARCHAR(10) NOT NULL,
       	INDEX idx_day (day),
		FOREIGN KEY (user_id) REFERENCES users(id)
    );`)

	config := fiber.Config{
		JSONEncoder: json.Marshal,
		JSONDecoder: json.Unmarshal,
	}

	s := fiber.New(config)

	s.Post("/checkin", func(c *fiber.Ctx) error {
		var checkinInput struct {
			Email string `json:"email"`
		}

		c.BodyParser(&checkinInput)

		if checkinInput.Email == "" {
			return c.Status(400).JSON(fiber.Map{
				"status":  "Bad Request",
				"message": "Email is required",
			})
		}

		_, err := mail.ParseAddress(checkinInput.Email)
		if err != nil {
			return c.Status(400).JSON(fiber.Map{
				"status":  "Bad Request",
				"message": "Invalid email",
			})
		}

		user, _ := CreateUser(db, checkinInput.Email)

		return c.Status(200).JSON(fiber.Map{
			"status":  "Success",
			"message": "Success",
			"data":    user,
		})
	})

	s.Get("/schedule", func(c *fiber.Ctx) error {
		email := c.Query("email")
		if email == "" {
			return c.Status(400).JSON(fiber.Map{
				"status":  "Bad Request",
				"message": "Email is required",
			})
		}

		_, err := mail.ParseAddress(email)
		if err != nil {
			return c.Status(400).JSON(fiber.Map{
				"status":  "Bad Request",
				"message": "Invalid email",
			})
		}

		user, err := GetUserDetails(db, email)
		if err != nil {
			if err == sql.ErrNoRows {
				return c.Status(404).JSON(fiber.Map{
					"status":  "Not Found",
					"message": "Email is not found",
				})
			}

			return c.Status(500).JSON(fiber.Map{
				"status":  "Internal Server Error",
				"message": "Something went wrong",
			})
		}

		day := c.Query("day")
		if day != "" {
			if _, ok := validDays[day]; !ok {
				return c.Status(400).JSON(fiber.Map{
					"status":  "Bad Request",
					"message": "Day is invalid",
				})
			}

			scheduleOnDay := GetScheduleOnDay(db, user.ID, day)

			return c.Status(200).JSON(fiber.Map{
				"status":  "Success",
				"message": "Success",
				"data":    scheduleOnDay,
			})
		}

		schedules := GetTotalSchedulesPerDays(db, user.ID)

		return c.Status(200).JSON(fiber.Map{
			"status":  "Success",
			"message": "Success",
			"data":    schedules,
		})
	})

	s.Post("/schedule", func(c *fiber.Ctx) error {
		email := c.Query("email")
		if email == "" {
			return c.Status(400).JSON(fiber.Map{
				"status":  "Bad Request",
				"message": "Email is required",
			})
		}

		_, err := mail.ParseAddress(email)
		if err != nil {
			return c.Status(400).JSON(fiber.Map{
				"status":  "Bad Request",
				"message": "Invalid email",
			})
		}

		var scheduleInput struct {
			Title string `json:"title"`
			Day   string `json:"day"`
		}

		c.BodyParser(&scheduleInput)

		if scheduleInput.Title == "" {
			return c.Status(400).JSON(fiber.Map{
				"status":  "Bad Request",
				"message": "Title is required",
			})
		}

		if scheduleInput.Day == "" {
			return c.Status(400).JSON(fiber.Map{
				"status":  "Bad Request",
				"message": "Day is required",
			})
		}

		if _, ok := validDays[scheduleInput.Day]; !ok {
			return c.Status(400).JSON(fiber.Map{
				"status":  "Bad Request",
				"message": "Day is invalid",
			})
		}

		user, err := GetUserDetails(db, email)
		if err != nil {
			if err == sql.ErrNoRows {
				return c.Status(404).JSON(fiber.Map{
					"status":  "Not Found",
					"message": "Email is not found",
				})
			}

			return c.Status(500).JSON(fiber.Map{
				"status":  "Internal Server Error",
				"message": "Something went wrong",
			})
		}

		schedule, _ := CreateSchedule(db, user.ID, scheduleInput.Title, scheduleInput.Day)

		return c.Status(201).JSON(fiber.Map{
			"status":  "Success",
			"message": "Success",
			"data":    schedule,
		})
	})

	s.Delete("/schedule", func(c *fiber.Ctx) error {
		email := c.Query("email")
		if email == "" {
			return c.Status(400).JSON(fiber.Map{
				"status":  "Bad Request",
				"message": "Email is required",
			})
		}

		_, err := mail.ParseAddress(email)
		if err != nil {
			return c.Status(400).JSON(fiber.Map{
				"status":  "Bad Request",
				"message": "Invalid email",
			})
		}

		id := c.QueryInt("id")

		user, err := GetUserDetails(db, email)
		if err != nil {
			if err == sql.ErrNoRows {
				return c.Status(404).JSON(fiber.Map{
					"status":  "Not Found",
					"message": "Email is not found",
				})
			}

			return c.Status(500).JSON(fiber.Map{
				"status":  "Internal Server Error",
				"message": "Something went wrong",
			})
		}

		schedule, err := GetScheduleDetails(db, id)
		if err != nil {
			return c.Status(404).JSON(fiber.Map{
				"status":  "Not Found",
				"message": fmt.Sprintf("Schedule with ID %d Not Found", id),
			})
		}

		if user.ID != schedule.UserID {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"status":  "Forbidden",
				"message": "Access denied!",
			})
		}

		go DeleteSchedule(db, id)

		return c.Status(200).JSON(fiber.Map{
			"status":  "Success",
			"message": "Success",
			"data":    struct{}{},
		})
	})

	s.Patch("/schedule", func(c *fiber.Ctx) error {
		email := c.Query("email")
		if email == "" {
			return c.Status(400).JSON(fiber.Map{
				"status":  "Bad Request",
				"message": "Email is required",
			})
		}

		_, err := mail.ParseAddress(email)
		if err != nil {
			return c.Status(400).JSON(fiber.Map{
				"status":  "Bad Request",
				"message": "Invalid email",
			})
		}

		id := c.QueryInt("id")

		user, err := GetUserDetails(db, email)
		if err != nil {
			if err == sql.ErrNoRows {
				return c.Status(404).JSON(fiber.Map{
					"status":  "Not Found",
					"message": "Email is not found",
				})
			}

			return c.Status(500).JSON(fiber.Map{
				"status":  "Internal Server Error",
				"message": "Something went wrong",
			})
		}

		schedule, _ := GetScheduleDetails(db, id)
		if schedule.ID == 0 {
			return c.Status(404).JSON(fiber.Map{
				"status":  "Not Found",
				"message": fmt.Sprintf("Schedule with ID %d Not Found", id),
			})
		}

		if schedule.UserID != user.ID {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"status":  "Forbidden",
				"message": "Access denied!",
			})
		}

		var scheduleInput struct {
			Title string `json:"title"`
		}

		c.BodyParser(&scheduleInput)

		if scheduleInput.Title == "" {
			return c.Status(400).JSON(fiber.Map{
				"status":  "Bad Request",
				"message": "Title is required",
			})
		}

		schedule.Title = scheduleInput.Title

		go UpdateSchedule(db, schedule.ID, schedule.Title, schedule.Day)

		return c.Status(201).JSON(fiber.Map{
			"status":  "Success",
			"message": "Success",
			"data":    schedule,
		})
	})

	if err := s.Listen(":3030"); err != nil {
		panic(err.Error())
	}
}

func GetScheduleDetails(db *sql.DB, id int) (Schedule, error) {
	var schedule Schedule
	row := db.QueryRow("SELECT id, title, day, user_id FROM schedules WHERE id = ?", id)
	if err := row.Scan(&schedule.ID, &schedule.Title, &schedule.Day, &schedule.UserID); err != nil {
		return Schedule{}, err
	}

	return schedule, nil
}

func UpdateSchedule(db *sql.DB, id int, title, day string) error {
	_, err := db.Exec("UPDATE schedules SET title = ?, day = ? WHERE id = ?", title, day, id)
	return err
}

func GetScheduleOnDay(db *sql.DB, id int, day string) []Schedule {
	rows, err := db.Query("SELECT id, user_id, title, day FROM schedules WHERE user_id = ? AND day = ?", id, day)
	if err != nil {
		return []Schedule{}
	}

	schedules := []Schedule{}
	for rows.Next() {
		var schedule Schedule
		if err := rows.Scan(&schedule.ID, &schedule.UserID, &schedule.Title, &schedule.Day); err != nil {
			panic(err.Error())
		}
		schedules = append(schedules, schedule)
	}

	return schedules
}

func getEnv(key, defaultVal string) string {
	value, ok := os.LookupEnv(key)
	if !ok {
		return defaultVal
	}
	return value
}

type User struct {
	ID    int    `json:"id"`
	Email string `json:"email"`
}

func CreateUser(db *sql.DB, email string) (User, error) {
	var user User

	stmtSelect, err := db.Prepare("SELECT id, email FROM users WHERE email = ?")
	if err != nil {
		return user, err
	}
	defer stmtSelect.Close()

	err = stmtSelect.QueryRow(email).Scan(&user.ID, &user.Email)
	if err != nil {
		if err == sql.ErrNoRows {
			stmtInsert, err := db.Prepare("INSERT INTO users (email) VALUES (?)")
			if err != nil {
				return user, err
			}
			defer stmtInsert.Close()

			res, err := stmtInsert.Exec(email)
			if err != nil {
				return user, err
			}
			id, err := res.LastInsertId()
			if err != nil {
				return user, err
			}
			user.ID = int(id)
			user.Email = email
			return user, nil
		}
		return user, err
	}
	return user, nil
}

type Schedule struct {
	ID     int    `json:"id"`
	UserID int    `json:"user_id"`
	Title  string `json:"title"`
	Day    string `json:"day"`
}

func CreateSchedule(db *sql.DB, userID int, title string, day string) (Schedule, error) {
	schedule := Schedule{
		UserID: userID,
		Title:  title,
		Day:    day,
	}

	stmt, err := db.Prepare("INSERT INTO schedules (user_id, title, day) VALUES (?, ?, ?)")
	if err != nil {
		return Schedule{}, err
	}
	defer stmt.Close()

	res, err := stmt.Exec(userID, title, day)
	if err != nil {
		return Schedule{}, err
	}

	id, err := res.LastInsertId()
	if err != nil {
		return Schedule{}, err
	}

	schedule.ID = int(id)
	return schedule, nil
}

func GetUserDetails(db *sql.DB, email string) (User, error) {
	var user User
	row := db.QueryRow("SELECT id, email FROM users WHERE email = ?", email)
	err := row.Scan(&user.ID, &user.Email)
	if err != nil {
		return User{}, err
	}
	return user, nil
}

func IsUserExists(db *sql.DB, email string) (exists bool) {
	query := "SELECT exists(SELECT 1 FROM users WHERE email = ?)"
	db.QueryRow(query, email).Scan(&exists)
	return exists
}
func GetTotalSchedulesPerDays(db *sql.DB, userId int) fiber.Map {
	schedulesTotal := fiber.Map{
		"monday":    0,
		"tuesday":   0,
		"wednesday": 0,
		"thursday":  0,
		"friday":    0,
	}

	rows, err := db.Query("SELECT day, COUNT(*) FROM schedules WHERE user_id = ? GROUP BY day", userId)
	if err != nil {
		return schedulesTotal
	}

	for rows.Next() {
		var day string
		var count int
		rows.Scan(&day, &count)

		switch day {
		case "monday":
			schedulesTotal["monday"] = count
		case "tuesday":
			schedulesTotal["tuesday"] = count
		case "wednesday":
			schedulesTotal["wednesday"] = count
		case "thursday":
			schedulesTotal["thursday"] = count
		case "friday":
			schedulesTotal["friday"] = count
		}
	}

	return schedulesTotal
}

func DeleteSchedule(db *sql.DB, id int) {
	db.Exec("DELETE FROM schedules WHERE id = ?", id)
}

var validDays = map[string]bool{
	"monday":    true,
	"tuesday":   true,
	"wednesday": true,
	"thursday":  true,
	"friday":    true,
}
