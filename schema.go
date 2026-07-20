package docent

import "reflect"

// Command is a framework-neutral schema IR node for one command in a CLI tree.
// Children are sorted by Name for deterministic, byte-stable output. The zero
// value is not meaningful; use adapters such as the cobra package to produce it.
type Command struct {
	// Name is the command's short name (the last path segment).
	Name string `json:"name"`

	// Path is the space-separated sequence of names from the root to this
	// command, e.g. "issue create".
	Path string `json:"path"`

	// Description is the short, one-line description of the command.
	Description string `json:"description,omitempty"`

	// Aliases lists alternative names the command answers to, sorted for
	// determinism. An agent that reads documentation mentioning an alias can
	// recognize it as this command.
	Aliases []string `json:"aliases,omitempty"`

	// Deprecated carries the framework's deprecation message when the
	// command is deprecated, empty otherwise. Agents should prefer the
	// replacement the message names instead of invoking this command.
	Deprecated string `json:"deprecated,omitempty"`

	// ReadOnly indicates the command performs no writes. Adapters set this
	// from framework-specific metadata; when unavailable it defaults to false.
	ReadOnly bool `json:"read_only,omitempty"`

	// Hidden indicates the command is not shown in help output.
	Hidden bool `json:"hidden,omitempty"`

	// Flags contains all flags available on this command, sorted by Name.
	Flags []Flag `json:"flags,omitempty"`

	// FlagGroups contains mutual-exclusion and required-together relations
	// among the command's flags, sorted for determinism.
	FlagGroups []FlagGroup `json:"flag_groups,omitempty"`

	// Extensions carries host-owned structured metadata — the slot for
	// everything the neutral IR deliberately does not model: auth
	// requirements, environment variables, exit codes, output contracts,
	// per-command examples. On the root command it describes the tool; on
	// any other node, that command. One documented exception to host
	// ownership: when Config.ContractVersion is set, the cobra adapter's
	// schema command stamps a "contract_version" entry onto the emitted
	// root at emission time, overwriting a host entry of that name — the
	// stored tree is never touched. Values must be JSON-marshalable;
	// emission sorts keys, and every API boundary deep-copies the map like
	// the schema fields.
	Extensions map[string]any `json:"extensions,omitempty"`

	// InputSchema is an optional JSON Schema object describing the command's
	// structured input. Nil when the framework does not supply schema metadata.
	InputSchema map[string]any `json:"input_schema,omitempty"`

	// OutputSchema is an optional JSON Schema object describing the command's
	// structured output. Nil when the framework does not supply schema metadata.
	OutputSchema map[string]any `json:"output_schema,omitempty"`

	// Children contains this command's direct subcommands, sorted by Name.
	Children []Command `json:"children,omitempty"`
}

// Clone returns a deep copy of c sharing no slice or map storage with it,
// recursing through Children. It is the boundary-copy primitive: lookups and
// emission-time rewrites (masking volatile defaults, stamping a contract
// version) clone first so the original tree is never mutated.
func (c Command) Clone() Command {
	return applyRegistry(nil, c)
}

// Flag is a framework-neutral flag definition. Default holds the value exactly
// as the framework reports it (no type coercion). Enum is non-nil only when the
// flag's value type exposes a finite set of allowed values.
type Flag struct {
	// Name is the long flag name without leading dashes.
	Name string `json:"name"`

	// Shorthand is the single-character flag alias, or empty if none.
	Shorthand string `json:"shorthand,omitempty"`

	// Description is the flag's usage string.
	Description string `json:"description,omitempty"`

	// Type is the value type as reported by the flag framework (e.g. "string",
	// "bool", "int", "stringSlice").
	Type string `json:"type"`

	// Default is the default value as a string. Empty means no default or the
	// zero value for the type.
	Default string `json:"default,omitempty"`

	// Enum lists the allowed values when the flag type restricts to a finite
	// set. Nil when the flag accepts arbitrary values.
	Enum []string `json:"enum,omitempty"`

	// Required is true when the flag must be set on every invocation.
	Required bool `json:"required,omitempty"`

	// Persistent is true when the flag is defined on this command and
	// available to every descendant. Each flag appears exactly once in the
	// tree — on the command that defines it — so inherited flags are not
	// repeated per command; a consumer resolves a command's full flag
	// surface by walking its ancestors' persistent flags. When a command's
	// local flag shares a name with an ancestor's persistent flag, the
	// nearest definition wins for that command.
	Persistent bool `json:"persistent,omitempty"`

	// Extensions carries structured per-flag metadata the neutral IR does
	// not model — placeholders, value hints, display grouping. Like
	// Command.Extensions the slot is host-owned, but adapters may contribute
	// documented, namespaced entries: the cobra adapter surfaces a gechr/clib
	// extras annotation under the "clib" key (with its enum field omitted —
	// that is hoisted to Enum). Values must be JSON-marshalable; emission
	// sorts keys, and every API boundary deep-copies the map.
	Extensions map[string]any `json:"extensions,omitempty"`
}

// FlagGroupKind distinguishes the supported flag group relations.
type FlagGroupKind string

const (
	// FlagGroupMutuallyExclusive marks a group where at most one flag may be
	// set in a single invocation.
	FlagGroupMutuallyExclusive FlagGroupKind = "mutually_exclusive"

	// FlagGroupRequiredTogether marks a group where either all flags must be
	// set or none may be set.
	FlagGroupRequiredTogether FlagGroupKind = "required_together"

	// FlagGroupOneRequired marks a group where at least one flag must be set
	// in every invocation.
	FlagGroupOneRequired FlagGroupKind = "one_required"
)

// FlagGroup is a named relation over a set of flag names, attached to the
// command whose invocation it constrains. A member name may resolve to an
// ancestor's persistent flag rather than one of the command's own — flags
// are emitted once, on their defining command.
type FlagGroup struct {
	// Kind is the relation type: mutually_exclusive, required_together, or
	// one_required.
	Kind FlagGroupKind `json:"kind"`

	// Flags is the sorted list of flag names in the group.
	Flags []string `json:"flags"`
}

// FindByPath searches cmd and its descendants for the node whose Path equals
// path. The comparison is exact and case-sensitive; path uses the
// space-separated form defined by Command.Path (e.g. "issue create").
//
// Returns the matching Command and true when found; the zero Command and false
// otherwise. The search is depth-first pre-order. The returned Command is a
// deep copy; mutating it — including its slice and map fields — cannot affect
// the tree that was searched.
func FindByPath(cmd Command, path string) (Command, bool) {
	if cmd.Path == path {
		return cmd.Clone(), true
	}

	for _, child := range cmd.Children {
		if found, ok := FindByPath(child, path); ok {
			return found, ok
		}
	}

	return Command{}, false
}

// SchemaRegistry maps command paths to host-provided input and output JSON
// Schema objects. Keys are Command.Path values (space-separated command names,
// e.g. "app issue create"). An entry with a nil field omits that schema side;
// only the non-nil side is applied.
//
// Hosts build a registry and call Apply to enrich a Command tree produced by
// an adapter (e.g. the cobra adapter's Tree function). The registry itself is
// never modified by Apply.
type SchemaRegistry map[string]CommandSchemas

// CommandSchemas holds the optional input and output JSON Schema objects for
// one command path.
type CommandSchemas struct {
	// Input is the JSON Schema object describing the command's structured input.
	// Nil when no input schema is registered for this command path.
	Input map[string]any

	// Output is the JSON Schema object describing the command's structured output.
	// Nil when no output schema is registered for this command path.
	Output map[string]any
}

// Apply walks cmd and its descendants, returning a new Command tree with the
// registered schemas attached. cmd is not modified. Commands whose Path has no
// registry entry are copied without schema changes. Schemas from the registry
// overwrite any schemas already present on a matched command.
func (r SchemaRegistry) Apply(cmd Command) Command {
	return applyRegistry(r, cmd)
}

// applyRegistry is the recursive worker for SchemaRegistry.Apply and, with a
// nil registry, the single deep-copy walker behind Command.Clone and
// FindByPath — one implementation so lookup and registry application cannot
// drift in copy semantics.
func applyRegistry(r SchemaRegistry, cmd Command) Command {
	out := copyCommandShallow(cmd)

	if schemas, ok := r[cmd.Path]; ok {
		if schemas.Input != nil {
			out.InputSchema = deepCopyMapAny(schemas.Input)
		}

		if schemas.Output != nil {
			out.OutputSchema = deepCopyMapAny(schemas.Output)
		}
	}

	if cmd.Children != nil {
		out.Children = make([]Command, len(cmd.Children))

		for i, child := range cmd.Children {
			out.Children[i] = applyRegistry(r, child)
		}
	}

	return out
}

// copyCommandShallow returns a fresh Command whose slices are new allocations
// copied from cmd. Children is always nil; the caller sets it after copying.
// This prevents Apply from aliasing the caller's original slice headers.
func copyCommandShallow(cmd Command) Command {
	out := cmd
	out.Children = nil

	if cmd.Aliases != nil {
		out.Aliases = make([]string, len(cmd.Aliases))
		copy(out.Aliases, cmd.Aliases)
	}

	if cmd.Flags != nil {
		out.Flags = make([]Flag, len(cmd.Flags))
		copy(out.Flags, cmd.Flags)

		for i := range out.Flags {
			if cmd.Flags[i].Enum != nil {
				out.Flags[i].Enum = make([]string, len(cmd.Flags[i].Enum))
				copy(out.Flags[i].Enum, cmd.Flags[i].Enum)
			}

			if cmd.Flags[i].Extensions != nil {
				out.Flags[i].Extensions = deepCopyMapAny(cmd.Flags[i].Extensions)
			}
		}
	}

	if cmd.FlagGroups != nil {
		out.FlagGroups = make([]FlagGroup, len(cmd.FlagGroups))
		copy(out.FlagGroups, cmd.FlagGroups)

		for i := range out.FlagGroups {
			if cmd.FlagGroups[i].Flags != nil {
				out.FlagGroups[i].Flags = make([]string, len(cmd.FlagGroups[i].Flags))
				copy(out.FlagGroups[i].Flags, cmd.FlagGroups[i].Flags)
			}
		}
	}

	if cmd.Extensions != nil {
		out.Extensions = deepCopyMapAny(cmd.Extensions)
	}

	if cmd.InputSchema != nil {
		out.InputSchema = deepCopyMapAny(cmd.InputSchema)
	}

	if cmd.OutputSchema != nil {
		out.OutputSchema = deepCopyMapAny(cmd.OutputSchema)
	}

	return out
}

// deepCopyMapAny returns a deep copy of m. All JSON-marshalable value types
// are handled: the JSON-native shapes (map[string]any, []any) take a fast
// path, and every other slice, map, or pointer — the typed containers hosts
// build schemas from as Go literals, like []string or map[string]string —
// is copied via reflection so no mutable storage is shared with the source.
func deepCopyMapAny(m map[string]any) map[string]any {
	if m == nil {
		return nil
	}

	out := make(map[string]any, len(m))

	for k, v := range m {
		out[k] = deepCopyAny(v)
	}

	return out
}

// deepCopyAny deep-copies a single JSON-compatible value. Only slices, maps,
// and pointers can alias mutable storage through an interface — every other
// kind (strings, numbers, bools, struct and array values) is copied by value
// when read back out — so those three kinds are the ones copied here.
func deepCopyAny(v any) any {
	switch t := v.(type) {
	case nil:
		return nil

	case map[string]any:
		return deepCopyMapAny(t)

	case []any:
		out := make([]any, len(t))

		for i, e := range t {
			out[i] = deepCopyAny(e)
		}

		return out

	default:
		return deepCopyReflect(reflect.ValueOf(v))
	}
}

// deepCopyReflect copies slices, maps, and pointers of any element type,
// recursing through nested containers. A pointer's pointee is copied as a
// whole value; a struct pointee's own reference fields are not traversed —
// JSON Schema objects carry no such values, and the JSON-native fast paths
// above handle everything json.Unmarshal can produce.
func deepCopyReflect(rv reflect.Value) any {
	// Deliberately not an exhaustive reflect.Kind switch: only these three
	// kinds can alias mutable storage; every other kind copies by value.
	kind := rv.Kind()

	if kind == reflect.Slice {
		if rv.IsNil() {
			return rv.Interface()
		}

		out := reflect.MakeSlice(rv.Type(), rv.Len(), rv.Len())
		for i := range rv.Len() {
			// A nil interface element copies as its zero value — the slot
			// MakeSlice already holds; reflect.ValueOf(nil) is not settable.
			if c := deepCopyAny(rv.Index(i).Interface()); c != nil {
				out.Index(i).Set(reflect.ValueOf(c))
			}
		}

		return out.Interface()
	}

	if kind == reflect.Map {
		if rv.IsNil() {
			return rv.Interface()
		}

		out := reflect.MakeMapWithSize(rv.Type(), rv.Len())
		for _, k := range rv.MapKeys() {
			c := deepCopyAny(rv.MapIndex(k).Interface())
			if c == nil {
				// SetMapIndex with the zero Value would delete the key;
				// store the element type's zero value to preserve it.
				out.SetMapIndex(k, reflect.New(rv.Type().Elem()).Elem())

				continue
			}

			out.SetMapIndex(k, reflect.ValueOf(c))
		}

		return out.Interface()
	}

	if kind == reflect.Pointer {
		if rv.IsNil() {
			return rv.Interface()
		}

		out := reflect.New(rv.Type().Elem())
		out.Elem().Set(reflect.ValueOf(deepCopyAny(rv.Elem().Interface())))

		return out.Interface()
	}

	// Copied by value on the way in; nothing to share.
	return rv.Interface()
}
