package challenges

import (
	"embed"
	"fmt"
)

//go:embed testdata
var data embed.FS

// Challenge represents a single editing challenge.
type Challenge struct {
	ID     string
	Title  string
	Hint   string
	Before string
	After  string
}

type challengeInfo struct {
	id    string
	title string
	hint  string
}

var registry = []challengeInfo{
	{"01_add_error_handling", "Add Error Handling", "o to open lines, type if err != nil { ... }. Use yp to duplicate error blocks."},
	{"02_named_fields", "Convert to Named Fields", "C to copy cursor down, f, to find comma, select word with b, then i to prepend field name."},
	{"03_add_context", "Thread context.Context", "f( to jump to parens, a to append. Use % then s to find Query and append Context."},
	{"04_extract_constant", "Extract a Constant", "%s256 to select every 256, c then type ringSize. Then O above the func and type const ringSize = 256."},
	{"05_map_literal", "Build a Map Literal", "Change m := make(map[int]string) to return map[int]string{, turn each m[400] = \"...\" into 400: \"...\", and replace return m with the closing }."},
	{"06_early_return", "Flip to an Early Return", "i to insert ! before the condition. xd the else branch's return up as a guard, delete the else, then select the old body and < to dedent."},
	{"07_defer_cleanup", "Add defer Cleanup", "o to add defer f.Close() after open. Then xd on each redundant f.Close() line."},
	{"08_sprintf", "Concatenate with Sprintf", "Change the import to fmt. Rebuild the return as fmt.Sprintf with %s/%d/%t verbs, then list name, count, ok as the args."},
	{"09_swap_lines", "Swap Two Lines", "xd to cut line, move to target with j/k, then p to paste."},
	{"10_range_loop", "Loop over a Range", "ci( to change the for header to _, n := range nums. Then xd the redundant n := nums[i] line."},
	{"11_struct_tags", "Struct Tags", "Select lines with x, Alt-s to split into cursors, $ to go to EOL, a to append."},
	{"12_extract_variable", "Extract a Variable", "Select expression with v or f/t motions, d to cut, O to open line above, type var =, p to paste."},
	{"13_logger", "Switch to a Logger", "Change the import to log. Turn each fmt.Println(msg, x) into log.Printf with a format string folding x in as %d/%v."},
	{"14_change_signature", "Change Function Signature", "ci( to change inside parens. Use o/A to add return type. c to change body."},
	{"15_unwrap_block", "Unwrap a Block", "xd on if and closing }, then select body with x and < to dedent."},
	{"16_split_struct", "Split Single-line Struct", "f{ to find brace, a Enter to split. Use f, to find commas and r Enter to split each field."},
	{"17_rename_multiple", "Rename Across Function", "% to select all, s then type items to sub-select, c then type entries."},
	{"18_move_function", "Move Function", "Select function with x (or maf for treesitter), d to cut, move to target, p to paste."},
	{"19_convert_if_to_switch", "Convert If-chain to Switch", "c to change if/else keywords to case/default. xd to remove braces. Use . to repeat."},
	{"20_wrap_error", "Wrap an Error", "Select nil, err with v, c to change to fmt.Errorf(..., err). Or use mi( and edit."},
}

// Load reads all embedded challenges and returns them in order.
func Load() ([]Challenge, error) {
	challenges := make([]Challenge, 0, len(registry))
	for _, r := range registry {
		before, err := data.ReadFile(fmt.Sprintf("testdata/%s/before.go", r.id))
		if err != nil {
			return nil, fmt.Errorf("loading %s/before.go: %w", r.id, err)
		}
		after, err := data.ReadFile(fmt.Sprintf("testdata/%s/after.go", r.id))
		if err != nil {
			return nil, fmt.Errorf("loading %s/after.go: %w", r.id, err)
		}
		challenges = append(challenges, Challenge{
			ID:     r.id,
			Title:  r.title,
			Hint:   r.hint,
			Before: string(before),
			After:  string(after),
		})
	}
	return challenges, nil
}
