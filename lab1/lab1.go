package main

import (
	"fmt"
	"time"
)

func DaysUntilNewYear(date time.Time) int {
	newYear := time.Date(date.Year()+1, time.January, 1, 0, 0, 0, 0, time.UTC)
	from := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, time.UTC)

	return int(newYear.Sub(from).Hours() / 24)
}

func main() {
	now := time.Now()
	days := DaysUntilNewYear(now)

	fmt.Printf("Сегодня: %s\n", now.Format("02.01.2006"))
	fmt.Printf("До Нового года осталось: %d дней\n", days)
}
