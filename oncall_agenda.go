package main

import (
	"io/ioutil"
	"os"
	"text/template"
	"time"

	pagerduty "github.com/PagerDuty/go-pagerduty"

	"regexp"

	"sort"

	"gopkg.in/yaml.v2"
)

var authToken = ""

const ReadableDateFormat = "Mon, 01/02/2006"
const ReadableTimeFormat = "3:04pm"
const ReadableDateTimeFormat = "Mon, 01/02/2006, 3:04pm"
const ISO8601Format = "2006-01-02T15:04:05-07:00"
const UTCFormat = "2006-01-02T15:04:05Z"

type PagerDutyConfig struct {
	Authtoken string
}

func (conf *PagerDutyConfig) init() {
	if contents, err := ioutil.ReadFile("/Users/rich/.pd.yml"); err != nil {
		panic(err)
	} else {
		confLocal := &PagerDutyConfig{}
		if err := yaml.Unmarshal(contents, confLocal); err != nil {
			panic(err)
		}
		conf.Authtoken = confLocal.Authtoken
	}
}

// XXX - add off work hours

type Incident struct {
	IncidentNum   uint
	CreatedAt     time.Time
	CreatedAtTime string
	Responder     string
	Status        string
	Description   string
	HtmlURL       string
	Body          string
}

func getIncidents(client *pagerduty.Client, fromDate time.Time, untilDate time.Time) []Incident {
	var incidents = []Incident{}

	var opts pagerduty.ListIncidentsOptions

	opts.Since = fromDate.Format(ISO8601Format)
	opts.Until = untilDate.Format(ISO8601Format)
	//opts.TimeZone = "UTC"
	opts.Includes = []string{"first_trigger_log_entries"}
	opts.Limit = 100

	// XXX - do pagination on this

	if pdIncidents, err := client.ListIncidents(opts); err != nil {
		panic(err)
	} else {
		for _, incident := range pdIncidents.Incidents {
			createdAt, _ := time.Parse(UTCFormat, incident.CreatedAt)
			incidents = append(incidents, Incident{
				incident.IncidentNumber,
				createdAt,
				createdAt.Format(ReadableTimeFormat),
				incident.LastStatusChangeBy.Summary,
				incident.Status,
				incident.FirstTriggerLogEntry.EventDetails["description"],
				incident.HTMLURL,
				incident.FirstTriggerLogEntry.Channel.Body})
		}
	}

	return incidents
}

type RepeatingIncident struct {
	Description string
	Amount      uint
}

type RepeatingIncidents []RepeatingIncident

func (slice RepeatingIncidents) Len() int {
	return len(slice)
}

func (slice RepeatingIncidents) Less(i, j int) bool {
	return slice[i].Amount < slice[j].Amount
}

func (slice RepeatingIncidents) Swap(i, j int) {
	slice[i], slice[j] = slice[j], slice[i]
}

func getRepeatingIncidents(incidents []Incident) RepeatingIncidents {
	var repeatingIncidents RepeatingIncidents

	incidentsByType := map[string]RepeatingIncident{}
	for _, incident := range incidents {
		strippedDescription := regexp.MustCompile("\\d+").ReplaceAllString(incident.Description, "*")
		if repeatingIncident, present := incidentsByType[strippedDescription]; present {
			repeatingIncident.Amount = repeatingIncident.Amount + 1
			incidentsByType[strippedDescription] = repeatingIncident
		} else {
			repeatingIncident := RepeatingIncident{strippedDescription, 1}
			incidentsByType[strippedDescription] = repeatingIncident
		}
	}

	// remove the nonrepeating incidents
	for _, repeatingIncident := range incidentsByType {
		if repeatingIncident.Amount > 1 {
			repeatingIncidents = append(repeatingIncidents, repeatingIncident)
		}
	}

	sort.Sort(sort.Reverse(repeatingIncidents))
	return repeatingIncidents
}

func GetLastFridayAt7PM() time.Time {
	// A Weekday specifies a day of the week (Sunday = 0, ...)
	nowDate := time.Now()
	daysFromLastFriday := (-2 - int(nowDate.Weekday())) % 7
	if daysFromLastFriday == 0 {
		daysFromLastFriday = -7
	}
	lastFriday := nowDate.AddDate(0, 0, daysFromLastFriday)
	return time.Date(lastFriday.Year(), lastFriday.Month(), lastFriday.Day(), 19, 0, 0, 0, nowDate.Location())
}

type Schedule struct {
	Username  string
	StartDate string
	EndDate   string
}

type WikiTemplateValues struct {
	LastWeekSchedules            map[string][]Schedule
	NextWeekSchedules            map[string][]Schedule
	IncidentsOfLastWeek          map[string][]Incident
	RepeatingIncidentsOfLastWeek RepeatingIncidents
}

func getRelevantSchedules(client *pagerduty.Client) []string {
	var scheduleIds []string

	var scheduleOpts pagerduty.ListSchedulesOptions
	if schedules, err := client.ListSchedules(scheduleOpts); err != nil {
		panic(err)
	} else {
		for _, schedule := range schedules.Schedules {
			if schedule.Name == "Engineering" || schedule.Name == "Systems" {
				scheduleIds = append(scheduleIds, schedule.ID)
			}
		}
	}

	return scheduleIds
}

func getFinalSchedules(client *pagerduty.Client, scheduleIds []string, fromDate time.Time, untilDate time.Time) map[string][]Schedule {
	var schedules = map[string][]Schedule{}

	var scheduleDetailsOpts pagerduty.GetScheduleOptions
	scheduleDetailsOpts.Since = fromDate.Format(ISO8601Format)
	scheduleDetailsOpts.Until = untilDate.Format(ISO8601Format)
	for _, scheduleId := range scheduleIds {
		if scheduleDetails, err := client.GetSchedule(scheduleId, scheduleDetailsOpts); err != nil {
			panic(err)
		} else {
			for _, entry := range scheduleDetails.FinalSchedule.RenderedScheduleEntries {
				startDate, _ := time.Parse(ISO8601Format, entry.Start)
				endDate, _ := time.Parse(ISO8601Format, entry.End)

				schedules[scheduleDetails.Name] = append(schedules[scheduleDetails.Name], Schedule{
					entry.User.Summary,
					startDate.Format(ReadableDateFormat),
					endDate.Format(ReadableDateFormat)})
			}
		}
	}

	return schedules
}

func main() {
	wikiTemplate := template.Must(template.ParseFiles("confluence_wiki.template"))
	var templateValues WikiTemplateValues

	config := &PagerDutyConfig{}
	config.init()

	lastFridayAt7PM := GetLastFridayAt7PM()
	thisFridayAt7PM := lastFridayAt7PM.AddDate(0, 0, 7)

	client := pagerduty.NewClient(config.Authtoken)

	incidents := getIncidents(client, lastFridayAt7PM, thisFridayAt7PM)

	repeatingIncidents := getRepeatingIncidents(incidents)
	templateValues.RepeatingIncidentsOfLastWeek = repeatingIncidents

	// loop through incidents and group them by date
	incidentsOfLastWeek := map[string][]Incident{}
	for _, incident := range incidents {
		dateString := incident.CreatedAt.Format(ReadableDateFormat)
		incidentsOfLastWeek[dateString] = append(incidentsOfLastWeek[dateString], incident)
	}
	templateValues.IncidentsOfLastWeek = incidentsOfLastWeek

	scheduleIds := getRelevantSchedules(client)

	// for each individual schedule, look back one week (ideally fri 7pm to fri 7pm)
	// XXX - ending at 5pm, since systems changes shifts at 5pm, but dev changes at 7pm
	templateValues.LastWeekSchedules = getFinalSchedules(client, scheduleIds,
		lastFridayAt7PM, thisFridayAt7PM.Add(-time.Hour*2))
	templateValues.NextWeekSchedules = getFinalSchedules(client, scheduleIds,
		thisFridayAt7PM, thisFridayAt7PM.AddDate(0, 0, 7).Add(-time.Hour*2))

	wikiTemplate.Execute(os.Stdout, templateValues)
}
