package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"github.com/mr55p-dev/gonk"
	"github.com/tj/go-naturaldate"
)

var apiUrl = "https://www.oneleisure.net/umbraco/api/activeintime/TimetableHelperApi"
var templates = make(map[string]*template.Template)
var cfg *Config

type Config struct {
	Port         int    `config:"port"`
	Host         string `config:"host,optional"`
	TemplateBase string `config:"template,optional"`
	Center       string `config:"center,optional"`
}

type ResTemplateData struct {
	Stamp   time.Time
	Results []SwimDate
}

func mapFilterToName(filt string) string {
	switch filt {
	case "lane":
		return "Lane Swim"
	default:
		return ""
	}
}

type ApiRequest struct {
	Name      string   `json:"Name"`
	Timetable []string `json:"TimetableNames"`
	FromDate  string   `json:"FromDate"`
	Days      int      `json:"Days"`
}

type SwimmingTimetable struct {
	Name        string `json:"Name"`
	Date        string `json:"Date"`
	Time        string `json:"Time"`
	Description string `json:"Description"`
	Duration    string `json:"Duration"`
}

type SwimDate struct {
	Name  string    `json:"name"`
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

type ApiResponse struct {
	SwimmingTimetable []SwimmingTimetable `json:"Swimming Timetable"`
}

func main() {
	cfg = &Config{
		Host:         "127.0.0.1",
		TemplateBase: "./templates",
		Center:       "Huntingdon",
	}
	err := gonk.LoadConfig(cfg, gonk.EnvLoader(""))
	if err != nil {
		panic(err)
	}

	templates["index"] = template.Must(template.ParseFiles(
		filepath.Join(cfg.TemplateBase, "template.html"),
	))
	templates["result"] = template.Must(template.ParseFiles(
		filepath.Join(cfg.TemplateBase, "result.html"),
	))

	http.HandleFunc("GET /", HandleIndex)
	http.HandleFunc("POST /swim", HandleSwim)
	http.Handle("GET /assets/", http.StripPrefix("/assets/", http.FileServer(http.Dir("./public"))))

	conn := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	fmt.Println("Starting server on", conn)
	if err := http.ListenAndServe(conn, nil); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
}

func HandleIndex(w http.ResponseWriter, r *http.Request) {
	_ = templates["index"].Execute(w, nil)
	return
}

func HandleSwim(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		http.Error(w, "Failed to parse request", http.StatusBadRequest)
	}
	duration := r.Form.Get("query")
	if duration == "" {
		duration = "today"
	}
	filter := r.Form.Get("filter")
	startDate, endDate := parseDate(duration)
	swims := getSwim(startDate, endDate, filter)
	err = templates["result"].Execute(w, ResTemplateData{
		Stamp:   startDate,
		Results: swims,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	return
}

func parseDate(duration string) (time.Time, time.Time) {
	startDate, err := naturaldate.Parse(duration, time.Now(), naturaldate.WithDirection(naturaldate.Future))
	if err != nil {
		panic(err)
	}
	startDate = startDate.Add(time.Hour + time.Second)
	endDate := startDate.AddDate(0, 0, 1)
	return startDate, endDate
}

func getSwimMock(time.Time, time.Time, string) []SwimDate {
	return []SwimDate{
		{
			Name:  "Lane swim",
			Start: time.Time{},
			End:   time.Time{},
		},
		{
			Name:  "Not lane swim",
			Start: time.Time{},
			End:   time.Time{},
		},
		{
			Name:  "Another swim",
			Start: time.Time{},
			End:   time.Time{},
		},
	}
}

func getSwim(startDate, endDate time.Time, filter string) []SwimDate {
	days := endDate.Sub(startDate).Hours() / 24
	bodyData := &ApiRequest{
		Name:      cfg.Center,
		Timetable: []string{"Swimming Timetable"},
		FromDate:  startDate.UTC().Format(time.RFC3339),
		Days:      int(math.Ceil(days)),
	}
	body, err := json.Marshal(bodyData)
	if err != nil {
		panic(err)
	}

	bodyRdr := bytes.NewReader(body)
	req, err := http.NewRequest(http.MethodPost, apiUrl, bodyRdr)
	if err != nil {
		panic(err)
	}
	req.Header.Add("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		panic(err)
	}

	defer res.Body.Close()
	resData, err := io.ReadAll(res.Body)
	if err != nil {
		panic(err)
	}
	if res.StatusCode != http.StatusOK {
		fmt.Printf("res.Status: %v\n", res.Status)
		fmt.Printf("string(resData): %v\n", string(resData))
		panic("Request failed")
	}

	var apiResponse ApiResponse
	err = json.Unmarshal(resData, &apiResponse)
	if err != nil {
		panic(err)
	}

	swims := apiResponse.SwimmingTimetable
	laneSwims := []SwimDate{}
	now := time.Now()
	filterName := mapFilterToName(filter)

	for _, swim := range swims {
		if filterName == "" || swim.Name == filterName {
			startTime, endTime := splitTimes(swim.Date, swim.Time)
			if now.After(startTime) {
				continue
			}
			laneSwims = append(laneSwims, SwimDate{
				Name:  swim.Name,
				Start: startTime,
				End:   endTime,
			})
		}
	}

	return laneSwims
}

func renderTable(startDate time.Time, swims []SwimDate) string {
	rows := make([][]string, 0, len(swims))
	for _, v := range swims {
		rows = append(rows, []string{
			v.Name,
			v.Start.Format("15:04"),
			v.End.Format("15:04"),
		})
	}

	s := lipgloss.NewStyle().Bold(true).MarginLeft(1)
	fmt.Println(s.Render(fmt.Sprintf("Swimming times for %s", startDate.Format("Monday January 02"))))
	t := table.New().
		Border(lipgloss.NormalBorder()).
		Headers("Type", "Starts", "Ends").
		Width(54).
		Rows(rows...)
	return t.String()
}

func parseTime(date, tm string) time.Time {
	out, err := time.ParseInLocation("02/01/2006 15:04", fmt.Sprintf("%s %s", date, tm), time.Local)
	if err != nil {
		panic(err)
	}
	return out
}

func splitTimes(date, tm string) (time.Time, time.Time) {
	s := strings.Split(tm, " - ")
	if len(s) != 2 {
		panic("Invalid time format")
	}
	return parseTime(date, s[0]), parseTime(date, s[1])
}
