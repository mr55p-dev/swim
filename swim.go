package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"io"
	"math"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"github.com/tj/go-naturaldate"
)

var apiUrl = "https://www.oneleisure.net/umbraco/api/activeintime/TimetableHelperApi"
var center = flag.String("center", "Huntingdon", "Name of the center")

var rootTemplate = template.Must(template.ParseFiles("./templates/template.html"))
var resultTemplate = template.Must(template.ParseFiles("./templates/result.html"))

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
	http.HandleFunc("GET /", HandleIndex)
	http.HandleFunc("POST /swim", HandleSwim)
	http.Handle("GET /assets/", http.StripPrefix("/assets/", http.FileServer(http.Dir("./public"))))

	fmt.Println("Starting server")
	if err := http.ListenAndServe("127.0.0.1:8080", nil); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
}

func HandleIndex(w http.ResponseWriter, r *http.Request) {
	fmt.Println("req")
	_ = rootTemplate.Execute(w, nil)
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
	err = resultTemplate.Execute(w, ResTemplateData{
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
		Name:      *center,
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
