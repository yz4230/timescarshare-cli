package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/yz4230/timescarshare-cli/internal/session"
	"github.com/yz4230/timescarshare-cli/internal/timescar"
)

var searchFlags struct {
	by   string
	page int
	json bool
}

var searchCmd = &cobra.Command{
	Use:   "search <word>",
	Short: "Search Times Car Share stations",
	Long: `Search stations on share.timescar.jp using the saved login session.

Use --by to choose the search box: address (default) or station name. Use
--page to read later result pages.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		mode, err := timescar.ParseMode(searchFlags.by)
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
		res, err := client.Search(cmd.Context(), args[0], mode, searchFlags.page)
		if err != nil {
			return err
		}
		if searchFlags.json {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(res)
		}
		printResult(res)
		return nil
	},
}

func printResult(res *timescar.Result) {
	if len(res.Stations) == 0 {
		fmt.Printf("no stations found for %q\n", res.Query)
		return
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "CODE\tNAME\tADDRESS")
	for _, s := range res.Stations {
		fmt.Fprintf(w, "%s\t%s\t%s\n", s.Code, s.Name, s.Address)
	}
	w.Flush()
	fmt.Printf("\n%d stations (page %d/%d)\n", res.Total, res.Page, res.PageCount)
}

func init() {
	searchCmd.Flags().StringVar(&searchFlags.by, "by", "address", "search box: address or station")
	searchCmd.Flags().IntVar(&searchFlags.page, "page", 1, "result page number")
	searchCmd.Flags().BoolVar(&searchFlags.json, "json", false, "output JSON")
	rootCmd.AddCommand(searchCmd)
}
