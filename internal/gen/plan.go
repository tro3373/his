// Package gen writes the daily time headings of a work log from Claude Code
// session logs and recorded meetings.
package gen

import (
	"cmp"
	"fmt"
	"maps"
	"math"
	"slices"
	"time"
)

// Activity is one message in a Claude Code session.
type Activity struct {
	At     time.Time
	Cwd    string
	Tag    string // project the working directory belongs to
	Repo   string
	Branch string
}

// Meeting is a recorded call already assigned to a project.
type Meeting struct {
	Start time.Time
	End   time.Time
	Tag   string
	Title string
}

// Kind tells what a segment of the day was spent on.
type Kind int

const (
	KindWork Kind = iota
	KindMeeting
	KindBreak
)

// TagHours is the time spent on one project, in quarter hours.
type TagHours struct {
	Tag      string
	Quarters int
}

// Segment is a stretch of the day that becomes one time heading.
type Segment struct {
	Start time.Time
	Kind  Kind
	Hours []TagHours
	Title string
}

// Day is the plan of one day: its segments in time order and when it ended.
type Day struct {
	Segments []Segment
	End      time.Time
}

const (
	binSize = 10 * time.Minute
	quarter = 15 * time.Minute
)

// Clock is a time of day as the offset from midnight, "15:04" in the config.
type Clock time.Duration

// UnmarshalText reads "15:04".
func (c *Clock) UnmarshalText(b []byte) error {
	t, err := time.Parse("15:04", string(b))
	if err != nil {
		return fmt.Errorf("time of day must be HH:MM: %w", err)
	}
	*c = Clock(time.Duration(t.Hour())*time.Hour + time.Duration(t.Minute())*time.Minute)
	return nil
}

// Lunch is the break taken every day at the same time.
type Lunch struct {
	Start Clock `mapstructure:"start"`
	End   Clock `mapstructure:"end"`
}

// DayRules is what shapes a day besides its activity.
type DayRules struct {
	Lunch   Lunch
	MinIdle time.Duration // how long a stretch without activity, lunch aside, must last to be a break
	Limit   time.Time     // the day ends no later than this, truncated to the quarter hour
	From    time.Time     // the settled part ends here; zero when nothing is settled
}

// PlanDay splits the day into segments.
// The day starts and ends at the first and last activity, truncated to the hour.
// A day with activity at or past rules.Limit ends at the limit instead,
// and no day ends after it.
// Lunch, meetings and idle stretches cut the day; every other stretch is shared
// among projects by how many 10 minute bins each had activity in.
// An idle stretch is rules.MinIdle or more, lunch aside, without activity or meeting:
// it becomes a break rather than work made up from the day's share.
// Only what follows rules.From is planned, so a rerun adds to the settled part:
// it returns false until an hour has passed since rules.From, which keeps a log
// to one new heading an hour, and when no hour of activity or meeting follows it.
// It also returns false when nothing in the day counts as work or meeting.
func PlanDay(date time.Time, rules DayRules, acts []Activity, meets []Meeting) (Day, bool) {
	acts = between(acts, rules.From, midnight(date).AddDate(0, 0, 1))
	meets = meetingsEndingAfter(meets, rules.From)
	if len(acts) == 0 && len(meets) == 0 {
		return Day{}, false
	}
	day0 := midnight(date)
	limit := rules.Limit.Truncate(quarter)
	if limit.Sub(rules.From) < time.Hour {
		return Day{}, false
	}
	p := dayPlan{
		lunch0:  day0.Add(time.Duration(rules.Lunch.Start)),
		lunch1:  day0.Add(time.Duration(rules.Lunch.End)),
		minIdle: rules.MinIdle,
	}
	start, end := span(acts, meets, date.Location())
	start = maxTime(start, rules.From)
	if within(start, p.lunch0, p.lunch1) {
		start = p.lunch1
	}
	if !start.Before(limit) {
		return Day{}, false
	}
	end = minTime(end, limit)
	if slices.ContainsFunc(acts, func(a Activity) bool { return !a.At.Before(limit) }) {
		end = limit
	}
	if end.After(p.lunch0) && !end.After(p.lunch1) && start.Before(p.lunch0) {
		end = p.lunch0
	}
	if !end.After(rules.From) {
		return Day{}, false
	}
	if !end.After(start) {
		end = minTime(start.Add(time.Hour), limit)
	}
	p.meetings = meetingSlots(meets, start, end)
	// Idleness needs the activity past the limit too: without it, the gap up to the limit looks like work.
	p.idle = p.idleSlots(binActivities(acts, p.lunch0, p.lunch1), start, end)
	p.bins = binActivities(between(acts, time.Time{}, limit), p.lunch0, p.lunch1)
	p.dayWeights = weigh(p.bins, time.Time{}, end.Add(24*time.Hour))

	var segs []Segment
	for _, c := range p.cuts(start, end) {
		seg := p.segmentAt(c.from, c.to)
		n := len(segs)
		if n == 0 {
			segs = append(segs, seg)
			continue
		}
		prev := segs[n-1]
		switch {
		case prev.Kind == KindWork && seg.Kind == KindWork:
			segs[n-1] = p.segmentAt(prev.Start, c.to)
		case prev.Kind == KindBreak && seg.Kind == KindBreak:
		case prev.Kind == KindMeeting && seg.Kind == KindMeeting && prev.Title == seg.Title && prev.Hours[0].Tag == seg.Hours[0].Tag:
			// A meeting cut at lunch is still one meeting.
			segs[n-1].Hours[0].Quarters += seg.Hours[0].Quarters
		default:
			segs = append(segs, seg)
		}
	}
	if !slices.ContainsFunc(segs, func(s Segment) bool { return len(s.Hours) > 0 }) {
		return Day{}, false
	}
	return Day{Segments: segs, End: end}, true
}

// dayPlan holds what PlanDay derives once and every segment looks up.
type dayPlan struct {
	lunch0, lunch1 time.Time
	minIdle        time.Duration
	meetings       []slot
	idle           []interval
	bins           map[time.Time][]string
	dayWeights     map[string]float64
}

// span returns when the day starts and ends: the first and last activity
// truncated to the hour, widened by meetings truncated to the quarter hour.
func span(acts []Activity, meets []Meeting, loc *time.Location) (time.Time, time.Time) {
	var first, last time.Time
	stretch := func(from, to time.Time) {
		if first.IsZero() || from.Before(first) {
			first = from
		}
		if to.After(last) {
			last = to
		}
	}
	for _, a := range acts {
		at := a.At.In(loc).Truncate(time.Hour)
		stretch(at, at)
	}
	for _, m := range meets {
		stretch(m.Start.In(loc).Truncate(quarter), m.End.In(loc).Truncate(quarter))
	}
	return first, last
}

func meetingsEndingAfter(meets []Meeting, from time.Time) []Meeting {
	var out []Meeting
	for _, m := range meets {
		if !m.End.Truncate(quarter).After(from) {
			continue
		}
		out = append(out, m)
	}
	return out
}

type slot struct {
	from, to time.Time
	meeting  Meeting
}

// meetingSlots truncates meetings to quarter hours and clips them to the day.
func meetingSlots(meets []Meeting, start, end time.Time) []slot {
	var slots []slot
	for _, m := range meets {
		from, to := m.Start.In(start.Location()).Truncate(quarter), m.End.In(start.Location()).Truncate(quarter)
		if !to.After(from) {
			to = from.Add(quarter)
		}
		from, to = maxTime(from, start), minTime(to, end)
		if !to.After(from) {
			continue
		}
		slots = append(slots, slot{from: from, to: to, meeting: m})
	}
	slices.SortFunc(slots, func(a, b slot) int { return a.from.Compare(b.from) })
	return slots
}

type interval struct{ from, to time.Time }

// idleSlots finds the breaks between activity: gaps of minIdle or more between
// the bins and meetings, rounded inward to quarter hours and clipped to the day.
// The gap before the first activity is not one: the day starts at that activity.
func (p dayPlan) idleSlots(bins map[time.Time][]string, start, end time.Time) []interval {
	busy := make([]interval, 0, len(bins)+len(p.meetings))
	for b := range bins {
		busy = append(busy, interval{b, b.Add(binSize)})
	}
	for _, m := range p.meetings {
		busy = append(busy, interval{m.from, m.to})
	}
	slices.SortFunc(busy, func(a, b interval) int { return a.from.Compare(b.from) })
	var out []interval
	for i := 0; i+1 < len(busy); i++ {
		from := ceilQuarter(busy[i].to)
		to := busy[i+1].from.Truncate(quarter)
		from, to = maxTime(from, start), minTime(to, end)
		if to.Sub(from)-overlap(from, to, p.lunch0, p.lunch1) < p.minIdle {
			continue
		}
		out = append(out, interval{from, to})
	}
	return out
}

func ceilQuarter(t time.Time) time.Time {
	if f := t.Truncate(quarter); !f.Equal(t) {
		return f.Add(quarter)
	}
	return t
}

func overlap(from, to, a, b time.Time) time.Duration {
	lo, hi := maxTime(from, a), minTime(to, b)
	if !hi.After(lo) {
		return 0
	}
	return hi.Sub(lo)
}

// cuts splits [start, end) at lunch, at every meeting and at every idle stretch.
func (p dayPlan) cuts(start, end time.Time) []interval {
	points := []time.Time{start, end}
	add := func(ts ...time.Time) {
		for _, t := range ts {
			if t.After(start) && t.Before(end) {
				points = append(points, t)
			}
		}
	}
	add(p.lunch0, p.lunch1)
	for _, s := range p.meetings {
		add(s.from, s.to)
	}
	for _, s := range p.idle {
		add(s.from, s.to)
	}
	slices.SortFunc(points, time.Time.Compare)
	points = slices.CompactFunc(points, time.Time.Equal)
	var out []interval
	for i := 0; i+1 < len(points); i++ {
		out = append(out, interval{points[i], points[i+1]})
	}
	return out
}

// segmentAt builds the segment for [from, to). A meeting wins over lunch and idleness.
// cuts never straddles a boundary, so a stretch is inside a slot or outside it.
func (p dayPlan) segmentAt(from, to time.Time) Segment {
	quarters := int(to.Sub(from) / quarter)
	inside := func(a, b time.Time) bool { return !from.Before(a) && !to.After(b) }
	for _, s := range p.meetings {
		if inside(s.from, s.to) {
			return Segment{Start: from, Kind: KindMeeting, Hours: []TagHours{{s.meeting.Tag, quarters}}, Title: s.meeting.Title}
		}
	}
	if inside(p.lunch0, p.lunch1) || slices.ContainsFunc(p.idle, func(s interval) bool { return inside(s.from, s.to) }) {
		return Segment{Start: from, Kind: KindBreak}
	}
	w := weigh(p.bins, from, to)
	if len(w) == 0 {
		w = p.dayWeights
	}
	if len(w) == 0 {
		// Only meetings that day: the stretch between them is not known to be work.
		return Segment{Start: from, Kind: KindBreak}
	}
	return Segment{Start: from, Kind: KindWork, Hours: allocateQuarters(quarters, w)}
}

// binActivities groups activity into 10 minute bins, dropping lunch.
// Each bin holds the projects that had activity in it.
func binActivities(acts []Activity, lunch0, lunch1 time.Time) map[time.Time][]string {
	bins := map[time.Time][]string{}
	for _, a := range acts {
		at := a.At.In(lunch0.Location())
		if within(at, lunch0, lunch1) {
			continue
		}
		b := at.Truncate(binSize)
		if !slices.Contains(bins[b], a.Tag) {
			bins[b] = append(bins[b], a.Tag)
		}
	}
	return bins
}

// weigh sums the bins in [from, to). A bin shared by n projects gives each 1/n.
// The bins are summed in time order: float sums depend on the order, and
// allocateQuarters breaks ties on the remainders, so another order can move a quarter.
func weigh(bins map[time.Time][]string, from, to time.Time) map[string]float64 {
	w := map[string]float64{}
	for _, b := range slices.SortedFunc(maps.Keys(bins), time.Time.Compare) {
		if !within(b, from, to) {
			continue
		}
		tags := bins[b]
		for _, tag := range tags {
			w[tag] += 1 / float64(len(tags))
		}
	}
	return w
}

// allocateQuarters shares quarters among projects by weight.
// Floors first, then hands the rest out by the largest remainder,
// so the result always sums to quarters. Projects that end up with 0 are dropped.
// The result is ordered by quarters, larger first, then by tag.
func allocateQuarters(quarters int, weights map[string]float64) []TagHours {
	var total float64
	for _, v := range weights {
		total += v
	}
	if total == 0 {
		return nil
	}
	type share struct {
		tag  string
		q    int
		frac float64
	}
	shares := make([]share, 0, len(weights))
	given := 0
	for tag, v := range weights {
		raw := float64(quarters) * v / total
		q := int(math.Floor(raw))
		given += q
		shares = append(shares, share{tag, q, raw - float64(q)})
	}
	slices.SortFunc(shares, func(a, b share) int {
		return cmp.Or(cmp.Compare(b.frac, a.frac), cmp.Compare(a.tag, b.tag))
	})
	for i := 0; given < quarters; i = (i + 1) % len(shares) {
		shares[i].q++
		given++
	}
	var out []TagHours
	for _, s := range shares {
		if s.q == 0 {
			continue
		}
		out = append(out, TagHours{s.tag, s.q})
	}
	slices.SortFunc(out, func(a, b TagHours) int {
		return cmp.Or(cmp.Compare(b.Quarters, a.Quarters), cmp.Compare(a.Tag, b.Tag))
	})
	return out
}

// within reports whether t is in [from, to).
func within(t, from, to time.Time) bool { return !t.Before(from) && t.Before(to) }

func maxTime(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
}

func minTime(a, b time.Time) time.Time {
	if a.Before(b) {
		return a
	}
	return b
}
