package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	"github.com/yz4230/timescarshare-cli/internal/session"
	"github.com/yz4230/timescarshare-cli/internal/timescar"
)

var availFlags struct {
	date string
	from string
	to   string
	json bool
}

var availabilityCmd = &cobra.Command{
	Use:   "availability <station-code>...",
	Short: "Show car availability at stations",
	Long: `Show which cars are free at the given stations.

Station codes come from the search command. --date defaults to today, --from
to the current hour and --to to twelve hours later; all times are JST. The
window may cross midnight (--to is then treated as the next day).`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		from, to, err := resolveWindow(availFlags.date, availFlags.from, availFlags.to)
		if err != nil {
			return err
		}
		sess, err := session.Load()
		if errors.Is(err, fs.ErrNotExist) {
			return errors.New("no saved session; run `login` first")
		}
		if err != nil {
			return err
		}
		client, err := timescar.New(sess)
		if err != nil {
			return err
		}
		res, err := client.Availability(cmd.Context(), args, from, to)
		if err != nil {
			return err
		}
		if availFlags.json {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(res)
		}
		printAvailability(res)
		return nil
	},
}

func resolveWindow(dateStr, fromStr, toStr string) (time.Time, time.Time, error) {
	loc := timescar.Tokyo
	now := time.Now().In(loc)
	day := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	if dateStr != "" {
		d, err := time.ParseInLocation("2006-01-02", dateStr, loc)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("--date: %w", err)
		}
		day = d
	}
	if fromStr == "" {
		fromStr = fmt.Sprintf("%02d:00", now.Hour())
	}
	if toStr == "" {
		toStr = fmt.Sprintf("%02d:00", (now.Hour()+12)%24)
	}
	from, err := parseClock(day, fromStr)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("--from: %w", err)
	}
	to, err := parseClock(day, toStr)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("--to: %w", err)
	}
	if !to.After(from) {
		to = to.Add(24 * time.Hour)
	}
	return from, to, nil
}

func parseClock(day time.Time, s string) (time.Time, error) {
	t, err := time.Parse("15:04", s)
	if err != nil {
		return time.Time{}, fmt.Errorf("want HH:MM, got %q", s)
	}
	return day.Add(time.Duration(t.Hour())*time.Hour + time.Duration(t.Minute())*time.Minute), nil
}

func printAvailability(res *timescar.Availability) {
	fmt.Printf("%s - %s\n\n", res.From.Format("2006-01-02 15:04"), res.To.Format("2006-01-02 15:04"))
	for _, st := range res.Stations {
		fmt.Printf("%s  %s\n", st.Code, st.Name)
		if len(st.Cars) == 0 {
			fmt.Println("  (no cars)")
			continue
		}
		w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
		fmt.Fprintln(w, "  CAR\tAVAILABLE")
		for _, car := range st.Cars {
			fmt.Fprintf(w, "  %s\t%s\n", car.Name, formatRanges(car.Available))
		}
		w.Flush()
		fmt.Println()
	}
}

func formatRanges(ranges []timescar.Range) string {
	if len(ranges) == 0 {
		return "-"
	}
	parts := make([]string, len(ranges))
	for i, r := range ranges {
		parts[i] = r.From.Format("15:04") + "-" + r.To.Format("15:04")
	}
	return strings.Join(parts, ", ")
}

func init() {
	availabilityCmd.Flags().StringVar(&availFlags.date, "date", "", "date YYYY-MM-DD (default today, JST)")
	availabilityCmd.Flags().StringVar(&availFlags.from, "from", "", "window start HH:MM (default current hour, JST)")
	availabilityCmd.Flags().StringVar(&availFlags.to, "to", "", "window end HH:MM (default 12 hours after --from)")
	availabilityCmd.Flags().BoolVar(&availFlags.json, "json", false, "output JSON")
	rootCmd.AddCommand(availabilityCmd)
}
