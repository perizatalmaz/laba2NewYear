// Command lab5 — HTTP-сервис, который сообщает количество календарных
// дней, оставшихся до ближайшего следующего Нового года.
//
// Использование:
//
//	lab5 [-addr :8080]
//
// Сервис предоставляет следующие endpoints:
//
//	GET /api/v1/days-until-new-year[?date=YYYY-MM-DD]
//	GET /healthz
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// dateLayout — формат даты, который API принимает и возвращает
// (календарная дата ISO 8601, например 2026-10-14).
const dateLayout = time.DateOnly

// DaysUntilNewYear возвращает количество календарных дней от даты t
// до 1 января следующего года.
//
// Используется только календарная дата t (год, месяц и день в часовом
// поясе t), время суток не учитывается. Для 1 января результат равен
// длине всего года (365 или 366), для 31 декабря — 1. Високосные годы
// учитываются пакетом time автоматически.
//
// Разница вычисляется в UTC, где каждые сутки длятся ровно 24 часа,
// поэтому переход на летнее/зимнее время не влияет на результат.
func DaysUntilNewYear(t time.Time) int {
	year, month, day := t.Date()

	from := time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
	newYear := time.Date(year+1, time.January, 1, 0, 0, 0, 0, time.UTC)

	return int(newYear.Sub(from).Hours() / 24)
}

// daysResponse — успешный ответ endpoint расчёта дней.
type daysResponse struct {
	Date    string `json:"date"`
	NewYear string `json:"new_year"`
	Days    int    `json:"days"`
}

// errorResponse возвращается, когда запрос не удаётся обработать.
type errorResponse struct {
	Error string `json:"error"`
}

// healthResponse — ответ endpoint проверки состояния сервиса.
type healthResponse struct {
	Status string `json:"status"`
}

// newHandler возвращает корневой HTTP-обработчик сервиса.
// Функция now возвращает текущее время и используется, когда в запросе
// не указана дата; тесты передают сюда «замороженные» часы.
func newHandler(now func() time.Time, logger *slog.Logger) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/days-until-new-year", daysHandler(now))
	mux.HandleFunc("GET /healthz", handleHealth)

	return logRequests(logger, mux)
}

// daysHandler возвращает обработчик, который сообщает количество дней
// до Нового года для даты из параметра запроса "date",
// или для текущей даты, если параметр не указан.
func daysHandler(now func() time.Time) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		date, err := dateFromQuery(r.URL.Query(), now)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
			return
		}

		newYear := time.Date(date.Year()+1, time.January, 1, 0, 0, 0, 0, time.UTC)
		writeJSON(w, http.StatusOK, daysResponse{
			Date:    date.Format(dateLayout),
			NewYear: newYear.Format(dateLayout),
			Days:    DaysUntilNewYear(date),
		})
	}
}

// dateFromQuery извлекает дату из параметра запроса "date".
// Если параметра нет, возвращает текущее время из now.
// Указанный, но пустой или некорректный параметр считается ошибкой.
func dateFromQuery(query url.Values, now func() time.Time) (time.Time, error) {
	if !query.Has("date") {
		return now(), nil
	}

	value := query.Get("date")
	date, err := time.Parse(dateLayout, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid date %q: expected format YYYY-MM-DD", value)
	}

	return date, nil
}

// handleHealth сообщает, что сервис работает.
func handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, healthResponse{Status: "ok"})
}

// writeJSON записывает v в ответ в формате JSON с указанным кодом статуса.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)

	// Код статуса уже отправлен, поэтому ошибку кодирования
	// (обычно это закрытое клиентом соединение) сообщить клиенту нельзя.
	_ = json.NewEncoder(w).Encode(v)
}

// statusRecorder оборачивает http.ResponseWriter, чтобы запомнить код статуса.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

// WriteHeader запоминает код статуса и передаёт его дальше.
func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

// logRequests — middleware, которое записывает в журнал каждый запрос
// и результат его обработки.
func logRequests(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

		next.ServeHTTP(rec, r)

		logger.Info("request",
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.String("query", r.URL.RawQuery),
			slog.Int("status", rec.status),
			slog.Duration("duration", time.Since(start)),
		)
	})
}

func main() {
	addr := flag.String("addr", ":8080", "HTTP listen address")
	flag.Parse()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	if err := run(*addr, logger); err != nil {
		logger.Error("server stopped with error", slog.Any("error", err))
		os.Exit(1)
	}
}

// run запускает HTTP-сервер на адресе addr и блокируется, пока сервер
// не завершится с ошибкой или процесс не получит сигнал прерывания
// или завершения.
func run(addr string, logger *slog.Logger) error {
	srv := &http.Server{
		Addr:              addr,
		Handler:           newHandler(time.Now, logger),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		logger.Info("server started", slog.String("addr", addr))
		errCh <- srv.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
	}

	logger.Info("shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return srv.Shutdown(shutdownCtx)
}
