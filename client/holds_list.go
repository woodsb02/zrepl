package client

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"

	"github.com/fatih/color"
	"github.com/pkg/errors"
	"github.com/spf13/pflag"

	"github.com/zrepl/zrepl/cli"
	"github.com/zrepl/zrepl/endpoint"
	"github.com/zrepl/zrepl/util/chainlock"
)

var holdsListFlags struct {
	Filter holdsFilterFlags
	Json   bool
}

var holdsCmdList = &cli.Subcommand{
	Use:             "list",
	Run:             doHoldsList,
	NoRequireConfig: true,
	Short:           "list holds and bookmarks",
	SetupFlags: func(f *pflag.FlagSet) {
		holdsListFlags.Filter.registerHoldsFilterFlags(f, "list")
		f.BoolVar(&holdsListFlags.Json, "json", false, "emit JSON")
	},
}

func doHoldsList(sc *cli.Subcommand, args []string) error {
	var err error
	ctx := context.Background()

	if len(args) > 0 {
		return errors.New("this subcommand takes no positional arguments")
	}

	q, err := holdsListFlags.Filter.Query()
	if err != nil {
		return errors.Wrap(err, "invalid filter specification on command line")
	}

	abstractions, errors, err := endpoint.ListAbstractionsStreamed(ctx, q)
	if err != nil {
		return err // context clear by invocation of command
	}

	var line chainlock.L
	var wg sync.WaitGroup
	defer wg.Wait()
	wg.Add(1)

	// print results
	go func() {
		defer wg.Done()
		enc := json.NewEncoder(os.Stdout)
		for a := range abstractions {
			func() {
				defer line.Lock().Unlock()
				if holdsListFlags.Json {
					enc.SetIndent("", "  ")
					if err := enc.Encode(abstractions); err != nil {
						panic(err)
					}
					fmt.Println()
				} else {
					fmt.Println(a)
				}
			}()
		}
	}()

	// print errors to stderr
	errorColor := color.New(color.FgRed)
	var errorsSlice []endpoint.ListAbstractionsError
	wg.Add(1)
	go func() {
		defer wg.Done()
		for err := range errors {
			func() {
				defer line.Lock().Unlock()
				errorsSlice = append(errorsSlice, err)
				errorColor.Fprintf(os.Stderr, "%s\n", err)
			}()
		}
	}()
	wg.Wait()
	if len(errorsSlice) > 0 {
		errorColor.Add(color.Bold).Fprintf(os.Stderr, "there were errors in listing the abstractions")
		return fmt.Errorf("")
	} else {
		return nil
	}

}
