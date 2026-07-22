package cobra

import (
	"encoding/json"
	"sort"
	"strings"

	"github.com/matcra587/docent"
	gocobra "github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// annotationMutuallyExclusive is the annotation cobra sets on each flag in a
// mutually-exclusive group (via cmd.MarkFlagsMutuallyExclusive). The value is
// a []string where each element is the space-separated list of all flags in
// the group. Defined as cobra's unexported mutuallyExclusiveAnnotation const.
const annotationMutuallyExclusive = "cobra_annotation_mutually_exclusive"

// annotationRequiredTogether is the annotation cobra sets on each flag in a
// required-together group (via cmd.MarkFlagsRequiredTogether). Defined as
// cobra's unexported requiredAsGroupAnnotation const.
const annotationRequiredTogether = "cobra_annotation_required_if_others_set"

// annotationOneRequired is the annotation cobra sets on each flag in a
// one-required group (via cmd.MarkFlagsOneRequired). Defined as cobra's
// unexported oneRequiredAnnotation const.
const annotationOneRequired = "cobra_annotation_one_required"

// annotationRequired is the annotation cobra and pflag use to mark a flag as
// individually required (via cmd.MarkFlagRequired). Referencing the exported
// cobra constant keeps this traceable if cobra renames the field.
const annotationRequired = gocobra.BashCompOneRequiredFlag

// AnnotationEnum is the pflag annotation key hosts set to declare a flag's
// finite set of allowed values when the flag's value type does not implement
// an Enum() method. Each annotation entry is one allowed value:
//
//	_ = fs.SetAnnotation("output", docentcobra.AnnotationEnum, []string{"json", "text"})
const AnnotationEnum = "docent.enum"

// annotationClibExtra is the pflag annotation key gechr/clib sets via its
// Extend function: a single entry holding a JSON object whose "enum" field
// lists the flag's allowed values. Read here without importing clib, so
// clib-annotated hosts surface enums with no double-annotation.
const annotationClibExtra = "clib.extra"

// AnnotationReadOnly is the cobra command annotation hosts set to mark a
// command as performing no writes. Any non-empty value marks the command
// read-only in the schema IR:
//
//	cmd.Annotations = map[string]string{docentcobra.AnnotationReadOnly: "true"}
const AnnotationReadOnly = "docent.readonly"

// enumProvider is satisfied by pflag.Value implementations that expose a
// finite set of allowed values. Host-defined custom flag types with an
// Enum() []string method satisfy it. Hosts that publish enum metadata via
// pflag annotations rather than a method are handled separately in
// convertFlag.
type enumProvider interface {
	Enum() []string
}

// Tree walks root and all of its subcommands and returns the framework-neutral
// Command IR rooted at the given command. Subcommands are in Children, sorted
// by name. Flags are sorted by name. Flag groups are sorted by their flag list.
//
// Nil root returns the zero-value Command.
func Tree(root *gocobra.Command) docent.Command {
	if root == nil {
		return docent.Command{}
	}

	return walkCmd(root, "")
}

// walkCmd recursively converts a cobra.Command into a docent.Command.
// parentPath is the space-separated path of ancestor names; empty for the root.
func walkCmd(cmd *gocobra.Command, parentPath string) docent.Command {
	path := buildPath(cmd.Name(), parentPath)

	return docent.Command{
		Name:        cmd.Name(),
		Path:        path,
		Description: cmd.Short,
		Aliases:     commandAliases(cmd),
		Deprecated:  cmd.Deprecated,
		ReadOnly:    cmd.Annotations[AnnotationReadOnly] != "",
		Hidden:      cmd.Hidden,
		Flags:       extractFlags(cmd),
		FlagGroups:  extractFlagGroups(cmd),
		Children:    walkChildren(cmd, path),
	}
}

// commandAliases returns a sorted copy of the command's aliases; the IR must
// not share slice storage with the walked cobra tree, and sorted output keeps
// the schema byte-stable regardless of declaration order.
func commandAliases(cmd *gocobra.Command) []string {
	if len(cmd.Aliases) == 0 {
		return nil
	}

	aliases := make([]string, len(cmd.Aliases))
	copy(aliases, cmd.Aliases)
	sort.Strings(aliases)

	return aliases
}

// buildPath returns the space-separated full path for a command, given its
// name and the path of its parent.
func buildPath(name, parentPath string) string {
	if parentPath == "" {
		return name
	}

	return parentPath + " " + name
}

// walkChildren converts all direct subcommands and returns them sorted by name.
func walkChildren(cmd *gocobra.Command, parentPath string) []docent.Command {
	subs := cmd.Commands()
	if len(subs) == 0 {
		return nil
	}

	children := make([]docent.Command, len(subs))
	for i, sub := range subs {
		children[i] = walkCmd(sub, parentPath)
	}

	sort.Slice(children, func(i, j int) bool {
		return children[i].Name < children[j].Name
	})

	return children
}

// extractFlags returns the flags cmd defines — local flags plus persistent
// flags declared on cmd itself — sorted by name, with the persistent ones
// marked. Inherited flags are deliberately absent: each flag appears exactly
// once in the emitted tree, on its defining command, so a deep tree does not
// repeat every ancestor's persistent flags on every node (on real hosts that
// repetition dominated the schema's size).
func extractFlags(cmd *gocobra.Command) []docent.Flag {
	// Cobra rebuilds these flag sets on every accessor call (LocalFlags
	// re-merges persistent flags each time); resolve each once for the walk.
	local := cmd.LocalFlags()
	persistent := cmd.PersistentFlags()

	var flags []docent.Flag

	local.VisitAll(func(f *pflag.Flag) {
		// Persistence is decided by pointer identity, not name: a command
		// may define the same name both local and persistent (local wins
		// for itself, persistent applies below), and a name match would
		// mislabel the local variant and lose the persistent one entirely.
		isPersistent := persistent.Lookup(f.Name) == f
		flags = append(flags, convertFlag(f, isPersistent))
	})

	// LocalFlags deduplicates by name with the local variant winning, so a
	// persistent flag shadowed by a same-name local is absent from it; emit
	// the persistent variant too — it is real surface for every descendant.
	persistent.VisitAll(func(f *pflag.Flag) {
		if local.Lookup(f.Name) != f {
			flags = append(flags, convertFlag(f, true))
		}
	})

	sort.Slice(flags, func(i, j int) bool {
		if flags[i].Name != flags[j].Name {
			return flags[i].Name < flags[j].Name
		}

		// A same-name pair is local + persistent; order the local first for
		// a stable, documented tie-break.
		return !flags[i].Persistent
	})

	return flags
}

// convertFlag converts a pflag.Flag to a docent.Flag. persistent indicates
// whether the flag is declared persistent on the defining command (available
// to every descendant).
//
// Enum values are populated from three sources in priority order:
//  1. The enumProvider interface on f.Value (host custom value types).
//  2. The docent.enum pflag annotation, one allowed value per entry — the
//     explicit bridge, which outranks any flag-library convention.
//  3. The gechr/clib extras annotation ("clib.extra"), read without
//     importing clib.
//
// All sources are sorted for determinism; the first source that yields
// values wins.
func convertFlag(f *pflag.Flag, persistent bool) docent.Flag {
	fl := docent.Flag{
		Name:        f.Name,
		Shorthand:   f.Shorthand,
		Description: f.Usage,
		Type:        f.Value.Type(),
		Default:     f.DefValue,
		Persistent:  persistent,
	}

	// A flag is required when cobra set the BashCompOneRequiredFlag annotation
	// (via cmd.MarkFlagRequired or pflag.FlagSet.MarkFlagRequired).
	if _, ok := f.Annotations[annotationRequired]; ok {
		fl.Required = true
	}

	// Extract allowed values from pflag.Value types that expose them.
	// Host-defined custom flag types with an Enum() []string method satisfy
	// this interface. Values are sorted for determinism so the IR is
	// byte-stable regardless of definition order.
	if ep, ok := f.Value.(enumProvider); ok {
		if raw := ep.Enum(); len(raw) > 0 {
			enum := make([]string, len(raw))
			copy(enum, raw)
			sort.Strings(enum)
			fl.Enum = enum
		}
	}

	// Fall back to the docent.enum annotation when the Value type does not
	// implement enumProvider — the bridge for hosts whose flag library keeps
	// enum metadata somewhere docent should not know about.
	if len(fl.Enum) == 0 {
		if vals := f.Annotations[AnnotationEnum]; len(vals) > 0 {
			enum := make([]string, len(vals))
			copy(enum, vals)
			sort.Strings(enum)
			fl.Enum = enum
		}
	}

	// Last, the clib extras blob: hosts built on gechr/clib already declare
	// enums there, and requiring a docent.enum duplicate would let the two
	// silently drift.
	extras := clibExtras(f)

	if len(fl.Enum) == 0 {
		if enum := clibEnumFrom(extras); len(enum) > 0 {
			sort.Strings(enum)
			fl.Enum = enum
		}
	}

	// Everything else in the blob — placeholder, value hints, display
	// grouping — has no neutral IR field; surface it under the documented
	// "clib" namespace so clib hosts lose nothing. The enum key is dropped:
	// it is modeled above and duplicating it would let the two drift. Zero
	// values are dropped too — clib serializes every extras field on every
	// flag, and across hundreds of flags those empty fields dominated the
	// schema's extension bytes on a real host.
	delete(extras, "enum")

	if thinned := thinZeroValues(extras); len(thinned) > 0 {
		fl.Extensions = map[string]any{"clib": thinned}
	}

	return fl
}

// thinZeroValues returns extras with zero-valued entries removed — empty
// strings, false, nil, and empty collections — recursing into nested
// objects. An absent key says everything an empty one does, and the schema
// is an agent artifact where every key is repeated token cost. Numbers
// survive even at zero: 0 is a value, not an absence marker. Returns nil
// when nothing survives so the caller drops the namespace entirely. The
// input comes from json.Unmarshal, so only JSON-native kinds appear.
func thinZeroValues(extras map[string]any) map[string]any {
	out := make(map[string]any, len(extras))

	for key, value := range extras {
		switch v := value.(type) {
		case nil:
		case string:
			if v != "" {
				out[key] = v
			}
		case bool:
			if v {
				out[key] = v
			}
		case []any:
			if len(v) > 0 {
				out[key] = v
			}
		case map[string]any:
			if thinned := thinZeroValues(v); len(thinned) > 0 {
				out[key] = thinned
			}
		default:
			out[key] = v
		}
	}

	if len(out) == 0 {
		return nil
	}

	return out
}

// clibExtras parses the clib extras annotation into a map. The value is one
// JSON object; a missing or malformed blob yields nil — schema emission must
// never fail on another library's metadata.
func clibExtras(f *pflag.Flag) map[string]any {
	vals := f.Annotations[annotationClibExtra]
	if len(vals) == 0 {
		return nil
	}

	var extras map[string]any
	if err := json.Unmarshal([]byte(vals[0]), &extras); err != nil {
		return nil
	}

	return extras
}

// clibEnumFrom extracts the enum list from parsed clib extras. Non-string
// entries disqualify the whole list — a half-usable enum would misrepresent
// the flag's contract.
func clibEnumFrom(extras map[string]any) []string {
	raw, ok := extras["enum"].([]any)
	if !ok {
		return nil
	}

	enum := make([]string, 0, len(raw))

	for _, v := range raw {
		s, ok := v.(string)
		if !ok {
			return nil
		}

		enum = append(enum, s)
	}

	return enum
}

// extractFlagGroups collects mutual-exclusion, required-together, and
// one-required groups from the cobra annotation system and returns them
// sorted for determinism.
func extractFlagGroups(cmd *gocobra.Command) []docent.FlagGroup {
	// The three annotation kinds read identical flag-set views; build the
	// name sets once — cobra re-merges the sets on every LocalFlags/Flags
	// call — and pass them to each collection pass.
	local := make(map[string]struct{})
	cmd.LocalFlags().VisitAll(func(f *pflag.Flag) {
		local[f.Name] = struct{}{}
	})

	merged := cmd.Flags()

	visible := make(map[string]struct{})
	merged.VisitAll(func(f *pflag.Flag) {
		visible[f.Name] = struct{}{}
	})

	var groups []docent.FlagGroup

	groups = append(groups, collectAnnotationGroups(merged, local, visible, annotationMutuallyExclusive, docent.FlagGroupMutuallyExclusive)...)
	groups = append(groups, collectAnnotationGroups(merged, local, visible, annotationRequiredTogether, docent.FlagGroupRequiredTogether)...)
	groups = append(groups, collectAnnotationGroups(merged, local, visible, annotationOneRequired, docent.FlagGroupOneRequired)...)

	return groups
}

// collectAnnotationGroups reads flag groups from a specific pflag annotation
// key. Each flag in a group carries the annotation value with the
// space-separated names of all flags in the group; this function deduplicates
// groups across flags so each unique group appears exactly once. merged is
// the command's merged flag set, local and visible the name sets built from
// its local and merged views.
//
// Cobra writes the annotation onto the shared flag objects, which every
// command whose merged flag set contains the object can see — so a group
// marked on one command would otherwise leak to the whole tree. A group is
// emitted only where every member is visible in the command's merged set and
// at least one member is defined locally: the marking command qualifies,
// unrelated commands that merely inherit a member do not.
func collectAnnotationGroups(merged *pflag.FlagSet, local, visible map[string]struct{}, annotKey string, kind docent.FlagGroupKind) []docent.FlagGroup {
	seen := make(map[string]struct{})

	var groups []docent.FlagGroup

	merged.VisitAll(func(f *pflag.Flag) {
		for _, groupStr := range f.Annotations[annotKey] {
			if _, ok := seen[groupStr]; ok {
				continue
			}

			seen[groupStr] = struct{}{}

			names := strings.Fields(groupStr)
			sort.Strings(names)

			anyLocal := false
			allVisible := true

			for _, name := range names {
				if _, ok := local[name]; ok {
					anyLocal = true
				}

				if _, ok := visible[name]; !ok {
					allVisible = false
				}
			}

			if !anyLocal || !allVisible {
				continue
			}

			groups = append(groups, docent.FlagGroup{
				Kind:  kind,
				Flags: names,
			})
		}
	})

	// Sort groups for determinism: compare their sorted flag lists.
	sort.Slice(groups, func(i, j int) bool {
		return strings.Join(groups[i].Flags, "\x00") < strings.Join(groups[j].Flags, "\x00")
	})

	return groups
}
