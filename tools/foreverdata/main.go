package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"time"
)

const usageText = `usage: go run ./tools/foreverdata <command> [options]

Commands:
  verify  validate the committed snapshot and manifest without network access
  check   compare the live source (or --source file) with the committed snapshot
  update  replace the snapshot after validation and print a bounded change summary
  rebuild-manifest  migrate the derived manifest offline after a reviewed tool change

Options for check and update:
  --source string          HTTPS URL or local file for check (default https://talentsforever.com/data.json)
  --accept-risky-change    allow a generated-date rollback or a large record-count decrease (update only)
  --fail-on-change         make check return status 2 when a change is found
`

func main() {
	os.Exit(runCLI(context.Background(), os.Args[1:], os.Stdout, os.Stderr, time.Now))
}

func runCLI(ctx context.Context, args []string, stdout, stderr io.Writer, now func() time.Time) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, usageText)
		return 1
	}

	command := args[0]
	flags := flag.NewFlagSet(command, flag.ContinueOnError)
	flags.SetOutput(stderr)
	source := flags.String("source", sourceURL, "HTTPS URL or local JSON file")
	acceptRisky := flags.Bool("accept-risky-change", false, "accept a rollback or large record-count decrease")
	failOnChange := flags.Bool("fail-on-change", false, "return status 2 when check finds a change")
	if err := flags.Parse(args[1:]); err != nil {
		return 1
	}
	if flags.NArg() != 0 {
		fmt.Fprintf(stderr, "unexpected arguments: %v\n", flags.Args())
		return 1
	}

	root, err := findRepositoryRoot()
	if err != nil {
		fmt.Fprintf(stderr, "forever data: %v\n", err)
		return 1
	}
	paths := repositoryPaths(root)

	switch command {
	case "verify":
		if *source != sourceURL || *acceptRisky || *failOnChange {
			fmt.Fprintln(stderr, "verify does not accept command options")
			return 1
		}
		state, err := readCommittedState(paths)
		if err != nil {
			fmt.Fprintf(stderr, "forever data verification failed: %v\n", err)
			return 1
		}
		fmt.Fprintf(stdout, "verified talentsforever snapshot %s (generated %s, %d bytes)\n",
			shortHash(state.manifest.Snapshot.RawSHA256), state.manifest.Source.Generated, state.manifest.Snapshot.Bytes)
		return 0

	case "rebuild-manifest":
		if *source != sourceURL || *acceptRisky || *failOnChange {
			fmt.Fprintln(stderr, "rebuild-manifest does not accept command options")
			return 1
		}
		state, err := rebuildCommittedManifest(paths)
		if err != nil {
			fmt.Fprintf(stderr, "forever data manifest rebuild failed: %v\n", err)
			return 1
		}
		fmt.Fprintf(stdout, "rebuilt %s at schema %d (snapshot %s)\n",
			manifestRelativePath, state.manifest.SchemaVersion, shortHash(state.manifest.Snapshot.RawSHA256))
		return 0

	case "check", "update":
		if command == "check" && *acceptRisky {
			fmt.Fprintln(stderr, "check does not accept --accept-risky-change")
			return 1
		}
		if command == "update" && *failOnChange {
			fmt.Fprintln(stderr, "update does not accept --fail-on-change")
			return 1
		}
		if command == "update" && *source != sourceURL {
			fmt.Fprintln(stderr, "update requires the canonical source URL; --source overrides are check-only")
			return 1
		}
		if command == "update" {
			release, err := acquireUpdateLock(paths)
			if err != nil {
				fmt.Fprintf(stderr, "forever data: cannot start update: %v\n", err)
				return 1
			}
			defer release()
		}
		current, missing, err := readCommittedStateIfPresent(paths)
		if err != nil {
			fmt.Fprintf(stderr, "forever data: committed state is invalid: %v\n", err)
			return 1
		}

		fetched, err := fetchSource(ctx, *source, defaultHTTPClient())
		if err != nil {
			fmt.Fprintf(stderr, "forever data: fetch failed: %v\n", err)
			return 1
		}
		candidate, err := makeCandidate(fetched, sourceURL, now().UTC())
		if err != nil {
			fmt.Fprintf(stderr, "forever data: source validation failed: %v\n", err)
			return 1
		}

		if !missing && candidate.manifest.Snapshot.DocumentSHA256 == current.manifest.Snapshot.DocumentSHA256 {
			if candidate.manifest.Snapshot.RawSHA256 == current.manifest.Snapshot.RawSHA256 {
				fmt.Fprintf(stdout, "talentsforever snapshot is up to date (%s, generated %s)\n",
					shortHash(current.manifest.Snapshot.RawSHA256), current.manifest.Source.Generated)
			} else {
				fmt.Fprintf(stdout, "talentsforever data is unchanged; source formatting differs (committed %s, fetched %s)\n",
					shortHash(current.manifest.Snapshot.RawSHA256), shortHash(candidate.manifest.Snapshot.RawSHA256))
			}
			return 0
		}

		if missing {
			fmt.Fprintf(stdout, "new talentsforever snapshot: generated %s, %d bytes, %s\n",
				candidate.manifest.Source.Generated, candidate.manifest.Snapshot.Bytes, shortHash(candidate.manifest.Snapshot.RawSHA256))
		} else {
			printChangeSummary(stdout, current, candidate)
		}

		if command == "check" {
			if *failOnChange {
				return 2
			}
			return 0
		}
		updated, risks, err := persistCandidate(paths, current, candidate, missing, *acceptRisky)
		if len(risks) != 0 {
			for _, risk := range risks {
				fmt.Fprintf(stderr, "refusing risky update: %s\n", risk)
			}
			fmt.Fprintln(stderr, "review the source, then rerun update with --accept-risky-change")
			return 1
		}
		if err != nil {
			fmt.Fprintf(stderr, "forever data: update failed: %v\n", err)
			return 1
		}
		if !updated {
			fmt.Fprintln(stdout, "talentsforever snapshot is up to date")
			return 0
		}
		fmt.Fprintf(stdout, "updated %s and %s\n", snapshotRelativePath, manifestRelativePath)
		return 0

	default:
		fmt.Fprintf(stderr, "unknown command %q\n\n%s", command, usageText)
		return 1
	}
}
