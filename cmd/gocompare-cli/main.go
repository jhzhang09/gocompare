package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"gocompare/internal/compare"
	"gocompare/internal/document"
	"gocompare/internal/extend"
	"gocompare/internal/hex"
	"gocompare/internal/htmlview"
	"gocompare/internal/media"
	"gocompare/internal/merge"
	"gocompare/internal/operations"
	"gocompare/internal/paginate"
	"gocompare/internal/pever"
	"gocompare/internal/picture"
	"gocompare/internal/regfile"
	"gocompare/internal/script"
	"gocompare/internal/workspace"
	"io"
	"os"
)

// run 返回 0 相同、1 不同、2 参数或读取失败。
func run(args []string, out, stderr io.Writer) int {
	if len(args) == 1 && (args[0] == "--help" || args[0] == "-h") {
		fmt.Fprintln(out, "Usage: gocompare-cli <left-file> <right-file>\nDirectory report: gocompare-cli dir [--filter pattern] [--compare-mode] [--compare-time] [--links record|follow] <left-dir> <right-dir>\nSync preview: gocompare-cli preview <mode> <left-dir> <right-dir>\nSync: gocompare-cli sync [--apply --backup-dir path] <left-dir> <right-dir>\nRestore: gocompare-cli restore <journal.json>\nMerge: gocompare-cli merge [--base file] [--output file] <left-file> <right-file>\nHex: gocompare-cli hex <left-file> <right-file>\nPicture: gocompare-cli picture <left-image> <right-image>\nMedia: gocompare-cli media <file>\nPE: gocompare-cli pe <left-file> <right-file>\nRegistry: gocompare-cli reg <left.reg> <right.reg>\nExtend: gocompare-cli extend <spec.json> <path> [path]\nGit: gocompare-cli difftool <left-file> <right-file> | mergetool [--base file] [--output file] <left-file> <right-file>\nScript: gocompare-cli script <file>\nReport: gocompare-cli report --title text --input file --output file\nPaginate: gocompare-cli paginate --width N --rows N --input file\nWorkspace: gocompare-cli workspace save <bundle> <session>...\nExit codes: 0 equal or clean, 1 different or conflicts, 2 error")
		return 0
	}
	if len(args) > 0 && args[0] == "hex" {
		return hexFiles(args[1:], out, stderr)
	}
	if len(args) > 0 && args[0] == "picture" {
		return pictureFiles(args[1:], out, stderr)
	}
	if len(args) > 0 && args[0] == "media" {
		return mediaFile(args[1:], out, stderr)
	}
	if len(args) > 0 && args[0] == "pe" {
		return peFiles(args[1:], out, stderr)
	}
	if len(args) > 0 && args[0] == "reg" {
		return regFiles(args[1:], out, stderr)
	}
	if len(args) > 0 && args[0] == "extend" {
		return extendFile(args[1:], out, stderr)
	}
	if len(args) > 0 && args[0] == "script" {
		return scriptFile(args[1:], out, stderr)
	}
	if len(args) > 0 && args[0] == "report" {
		return reportFile(args[1:], out, stderr)
	}
	if len(args) > 0 && args[0] == "paginate" {
		return paginateFile(args[1:], out, stderr)
	}
	if len(args) > 0 && args[0] == "workspace" {
		return workspaceCmd(args[1:], out, stderr)
	}
	if len(args) > 0 && args[0] == "mergetool" {
		return mergeFiles(args[1:], out, stderr, true)
	}
	if len(args) > 0 && args[0] == "difftool" {
		args = args[1:]
	}
	if len(args) > 0 && args[0] == "restore" {
		if len(args) != 2 {
			fmt.Fprintln(stderr, "Usage: gocompare-cli restore <journal.json>")
			return 2
		}
		report, err := operations.Restore(context.Background(), args[1])
		if writeErr := json.NewEncoder(out).Encode(report); writeErr != nil {
			return 2
		}
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		return 0
	}
	if len(args) > 0 && args[0] == "merge" {
		return mergeFiles(args[1:], out, stderr, false)
	}
	if len(args) > 0 && args[0] == "sync" {
		flags := flag.NewFlagSet("sync", flag.ContinueOnError)
		flags.SetOutput(stderr)
		mode := flags.String("mode", "update-right", "sync mode")
		apply := flags.Bool("apply", false, "execute the previewed operations")
		backup := flags.String("backup-dir", "", "backup and journal directory outside both roots")
		if err := flags.Parse(args[1:]); err != nil {
			return 2
		}
		if flags.NArg() != 2 {
			fmt.Fprintln(stderr, "Usage: gocompare-cli sync [--mode mode] [--apply --backup-dir path] <left-dir> <right-dir>")
			return 2
		}
		if *apply && *backup == "" {
			fmt.Fprintln(stderr, "--backup-dir is required with --apply")
			return 2
		}
		p, err := operations.Preview(context.Background(), flags.Arg(0), flags.Arg(1), operations.Mode(*mode))
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		if !*apply {
			if err = json.NewEncoder(out).Encode(p); err != nil {
				return 2
			}
			if len(p.Actions) > 0 {
				return 1
			}
			return 0
		}
		report, err := operations.Execute(context.Background(), p, *backup)
		writeErr := json.NewEncoder(out).Encode(report)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		if writeErr != nil {
			return 2
		}
		return 0
	}
	if len(args) == 4 && args[0] == "preview" {
		plan, err := operations.Preview(context.Background(), args[2], args[3], operations.Mode(args[1]))
		if err != nil {
			fmt.Fprintln(stderr, "GoCompare:", err)
			return 2
		}
		if err = json.NewEncoder(out).Encode(plan); err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		if len(plan.Actions) > 0 {
			return 1
		}
		return 0
	}
	if len(args) > 0 && args[0] == "dir" {
		return dirFiles(args[1:], out, stderr)
	}
	if len(args) != 2 {
		fmt.Fprintln(stderr, "Usage: gocompare-cli <left-file> <right-file>")
		return 2
	}
	equal, err := compare.EqualFiles(context.Background(), args[0], args[1])
	if err != nil {
		fmt.Fprintln(stderr, "GoCompare:", err)
		return 2
	}
	if equal {
		fmt.Fprintln(out, "equal")
		return 0
	}
	fmt.Fprintln(out, "different")
	return 1
}

// mergeFiles 实现 merge 与 mergetool。merge 的输出只能是新文件；mergetool 遵循 Git 约定，
// $MERGED 在调用前已经存在，因此按刚读到的基线替换它，期间被其他程序改动则拒绝。
func mergeFiles(args []string, out, stderr io.Writer, replaceOutput bool) int {
	flags := flag.NewFlagSet("merge", flag.ContinueOnError)
	flags.SetOutput(stderr)
	base := flags.String("base", "", "common ancestor")
	outputHelp := "new output file; existing files are refused"
	if replaceOutput {
		outputHelp = "output file; an existing file (Git's $MERGED) is replaced"
	}
	output := flags.String("output", "", outputHelp)
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() != 2 {
		fmt.Fprintln(stderr, "Usage: gocompare-cli merge [--base file] [--output file] <left-file> <right-file>")
		return 2
	}
	ctx := context.Background()
	left, err := document.ReadDetected(ctx, flags.Arg(0))
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	right, err := document.ReadDetected(ctx, flags.Arg(1))
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	var result merge.Result
	if *base == "" {
		result, err = merge.MergeTwo(left.Text, right.Text)
	} else {
		ancestor, readErr := document.ReadDetected(ctx, *base)
		if readErr != nil {
			fmt.Fprintln(stderr, readErr)
			return 2
		}
		result, err = merge.MergeThree(ancestor.Text, left.Text, right.Text)
	}
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	text := result.Text()
	if *output == "" {
		fmt.Fprint(out, text)
	} else if err = writeMergeOutput(ctx, left, *output, text, replaceOutput); err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	if result.ConflictCount() > 0 {
		fmt.Fprintf(stderr, "%d conflicts\n", result.ConflictCount())
		return 1
	}
	return 0
}

func writeMergeOutput(ctx context.Context, left *document.File, path, text string, replace bool) error {
	if !replace {
		_, err := left.SaveAs(ctx, path, text)
		return err
	}
	var baseline *document.File
	if _, err := os.Lstat(path); err == nil {
		if baseline, err = document.ReadDetected(ctx, path); err != nil {
			return err
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	_, err := left.SaveAsOverwrite(ctx, path, text, baseline)
	return err
}

func dirFiles(args []string, out, stderr io.Writer) int {
	flags := flag.NewFlagSet("dir", flag.ContinueOnError)
	flags.SetOutput(stderr)
	filter := flags.String("filter", "", "relative path pattern")
	mode := flags.Bool("compare-mode", false, "compare permissions")
	mtime := flags.Bool("compare-time", false, "compare modification time")
	links := flags.String("links", "record", "record or follow")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() != 2 {
		fmt.Fprintln(stderr, "Usage: gocompare-cli dir [--filter pattern] [--compare-mode] [--compare-time] [--links record|follow] <left-dir> <right-dir>")
		return 2
	}
	rows, err := compare.CompareDirectories(context.Background(), flags.Arg(0), flags.Arg(1), compare.ScanOptions{
		Filter:      *filter,
		Links:       compare.LinkPolicy(*links),
		CompareMode: *mode,
		CompareTime: *mtime,
	})
	if err != nil {
		fmt.Fprintln(stderr, "GoCompare:", err)
		return 2
	}
	if err = json.NewEncoder(out).Encode(rows); err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	code := 0
	for _, row := range rows {
		if row.Status == compare.Failed {
			return 2
		}
		if row.Status != compare.Same {
			code = 1
		}
	}
	return code
}

func extendFile(args []string, out, stderr io.Writer) int {
	if len(args) < 2 {
		fmt.Fprintln(stderr, "Usage: gocompare-cli extend <spec.json> <path> [path]")
		return 2
	}
	spec, err := extend.Load(args[0])
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	var paths extend.Paths
	switch spec.Kind {
	case "compare":
		if len(args) != 3 {
			fmt.Fprintln(stderr, "Usage: gocompare-cli extend <spec.json> <left> <right>")
			return 2
		}
		paths.Left, paths.Right = args[1], args[2]
	case "convert":
		if len(args) != 2 {
			fmt.Fprintln(stderr, "Usage: gocompare-cli extend <spec.json> <input>")
			return 2
		}
		paths.In = args[1]
	case "compress":
		if len(args) != 3 {
			fmt.Fprintln(stderr, "Usage: gocompare-cli extend <spec.json> <input> <output>")
			return 2
		}
		paths.In, paths.Out = args[1], args[2]
	default:
		fmt.Fprintln(stderr, "unknown extension kind")
		return 2
	}
	result, err := extend.Execute(context.Background(), spec, paths)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	if spec.Kind == "compress" {
		fmt.Fprintf(out, "packed %d bytes\n", len(result.Payload))
		return 0
	}
	if _, err = out.Write(result.Payload); err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	return result.Code
}

func peFiles(args []string, out, stderr io.Writer) int {
	if len(args) != 2 {
		fmt.Fprintln(stderr, "Usage: gocompare-cli pe <left-file> <right-file>")
		return 2
	}
	left, err := readLimitedFile(args[0])
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	right, err := readLimitedFile(args[1])
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	lv, err := pever.FileVersion(left)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	rv, err := pever.FileVersion(right)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	fmt.Fprintf(out, "left %s\nright %s\n", lv, rv)
	if lv != rv {
		return 1
	}
	return 0
}

func regFiles(args []string, out, stderr io.Writer) int {
	if len(args) != 2 {
		fmt.Fprintln(stderr, "Usage: gocompare-cli reg <left.reg> <right.reg>")
		return 2
	}
	left, err := readLimitedFile(args[0])
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	right, err := readLimitedFile(args[1])
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	leftText, err := regfile.Decode(left)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	rightText, err := regfile.Decode(right)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	lv, err := regfile.Parse(leftText)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	rv, err := regfile.Parse(rightText)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	diffs := regfile.Diff(lv, rv)
	if len(diffs) == 0 {
		fmt.Fprintln(out, "equal")
		return 0
	}
	for _, line := range diffs {
		fmt.Fprintln(out, line)
	}
	return 1
}

func mediaFile(args []string, out, stderr io.Writer) int {
	if len(args) != 1 {
		fmt.Fprintln(stderr, "Usage: gocompare-cli media <file>")
		return 2
	}
	tags, err := media.Open(context.Background(), args[0])
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	fmt.Fprintf(out, "title %s\nartist %s\nalbum %s\n", tags.Title, tags.Artist, tags.Album)
	return 0
}

func pictureFiles(args []string, out, stderr io.Writer) int {
	if len(args) != 2 {
		fmt.Fprintln(stderr, "Usage: gocompare-cli picture <left-image> <right-image>")
		return 2
	}
	left, err := picture.Open(args[0])
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	right, err := picture.Open(args[1])
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	diff := picture.FirstDifference(left, right)
	if !diff.SameSize {
		fmt.Fprintf(out, "size %dx%d %dx%d\n", left.Bounds().Dx(), left.Bounds().Dy(), right.Bounds().Dx(), right.Bounds().Dy())
		return 1
	}
	if diff.Found {
		fmt.Fprintf(out, "pixel %d,%d\n", diff.X, diff.Y)
		return 1
	}
	fmt.Fprintln(out, "equal")
	return 0
}

func hexFiles(args []string, out, stderr io.Writer) int {
	if len(args) != 2 {
		fmt.Fprintln(stderr, "Usage: gocompare-cli hex <left-file> <right-file>")
		return 2
	}
	left, err := readLimitedFile(args[0])
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	right, err := readLimitedFile(args[1])
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	chunks := hex.Diff(left, right, 16)
	fmt.Fprint(out, hex.FormatDiff(chunks))
	if len(chunks) > 0 {
		return 1
	}
	return 0
}

func scriptFile(args []string, out, stderr io.Writer) int {
	if len(args) != 1 {
		fmt.Fprintln(stderr, "Usage: gocompare-cli script <file>")
		return 2
	}
	data, err := readLimitedFile(args[0])
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	code, err := script.Execute(context.Background(), string(data), out)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	return code
}

func reportFile(args []string, out, stderr io.Writer) int {
	flags := flag.NewFlagSet("report", flag.ContinueOnError)
	flags.SetOutput(stderr)
	title := flags.String("title", "Report", "report title")
	input := flags.String("input", "", "source text or html")
	output := flags.String("output", "", "new html file")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if *input == "" || *output == "" {
		fmt.Fprintln(stderr, "Usage: gocompare-cli report --title text --input file --output file")
		return 2
	}
	data, err := readLimitedFile(*input)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	blocks := htmlview.Blocks(string(data))
	if len(blocks) == 0 {
		blocks = []htmlview.Block{{Tag: "p", Text: string(data)}}
	}
	f, err := os.OpenFile(*output, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	err = htmlview.WriteReport(f, *title, blocks)
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	return 0
}

func paginateFile(args []string, out, stderr io.Writer) int {
	flags := flag.NewFlagSet("paginate", flag.ContinueOnError)
	flags.SetOutput(stderr)
	width := flags.Int("width", 40, "columns")
	rows := flags.Int("rows", 20, "rows per page")
	input := flags.String("input", "", "text file")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if *input == "" {
		fmt.Fprintln(stderr, "Usage: gocompare-cli paginate --width N --rows N --input file")
		return 2
	}
	data, err := readLimitedFile(*input)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	pages := paginate.Pages(string(data), *width, *rows)
	for i, page := range pages {
		if i > 0 {
			fmt.Fprint(out, "\f")
		}
		fmt.Fprint(out, page)
	}
	return 0
}

func workspaceCmd(args []string, out, stderr io.Writer) int {
	if len(args) < 2 {
		fmt.Fprintln(stderr, "Usage: gocompare-cli workspace save <bundle> <session>... | workspace recover <bundle>")
		return 2
	}
	switch args[0] {
	case "save":
		if len(args) < 3 {
			fmt.Fprintln(stderr, "workspace save needs a bundle and at least one session")
			return 2
		}
		var sessions []workspace.Session
		for _, path := range args[2:] {
			s, err := workspace.Read(path)
			if err != nil {
				fmt.Fprintln(stderr, err)
				return 2
			}
			sessions = append(sessions, s)
		}
		if err := workspace.Autosave(args[1], sessions); err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		return 0
	case "recover":
		bundle, err := workspace.Recover(args[1])
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		if err = json.NewEncoder(out).Encode(bundle); err != nil {
			return 2
		}
		return 0
	default:
		fmt.Fprintln(stderr, "unknown workspace command")
		return 2
	}
}

func readLimitedFile(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, (16<<20)+1))
	if err != nil {
		return nil, err
	}
	if len(data) > 16<<20 {
		return nil, fmt.Errorf("%s exceeds 16 MiB", path)
	}
	return data, nil
}

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }
