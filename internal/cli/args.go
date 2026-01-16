package cli

import (
	"fmt"
	"strconv"
	"strings"
)

type CmdType int

const (
	CmdGlobal CmdType = iota
	CmdInterleaved
	CmdAction
)

type ValueType int

const (
	ValueNone ValueType = iota
	ValueAny
	ValueNumber
	ValueOptional
)

type CommandDef struct {
	Name      string
	Short     string
	Type      CmdType
	ValueType ValueType
	Usage     string // Short description for -h
	Long      string // Detailed description for --help
	Internal  bool
	Category  string
}

type Op struct {
	Action  string
	Value   string
	Type    CmdType
	IsShort bool
}

type ParsedArgs struct {
	Globals map[string]string
	Ops     []Op
}

func Parse(args []string, defs []CommandDef) (*ParsedArgs, error) {
	longMap := make(map[string]CommandDef)
	shortMap := make(map[rune]CommandDef)

	for _, d := range defs {
		longMap[d.Name] = d
		if d.Short != "" {
			shortMap[[]rune(d.Short)[0]] = d
		}
	}

	res := &ParsedArgs{
		Globals: make(map[string]string),
		Ops:     make([]Op, 0, len(args)),
	}

	for i := 0; i < len(args); i++ {
		arg := args[i]

		if arg == "--" {
			for j := i + 1; j < len(args); j++ {
				res.Ops = append(res.Ops, Op{Action: "FILE", Value: args[j], Type: CmdAction})
			}
			break
		}

		if strings.HasPrefix(arg, "--") {
			consumed, err := parseLong(arg, args, i, longMap, res)
			if err != nil {
				return nil, err
			}
			i += consumed
			continue
		}

		if strings.HasPrefix(arg, "-") && len(arg) > 1 {
			consumed, err := parseShort(arg, args, i, shortMap, res)
			if err != nil {
				return nil, err
			}
			i += consumed
			continue
		}

		res.Ops = append(res.Ops, Op{Action: "FILE", Value: arg, Type: CmdAction})
	}

	return res, nil
}

func parseLong(arg string, args []string, idx int, defs map[string]CommandDef, res *ParsedArgs) (int, error) {
	raw := strings.TrimLeft(arg, "-")
	key, val := raw, ""
	hasEq := false

	if i := strings.IndexByte(raw, '='); i != -1 {
		key, val = raw[:i], raw[i+1:]
		hasEq = true
	}

	def, ok := defs[key]
	if !ok {
		if key == "help" {
			res.Globals["help"] = "true"
			res.Ops = append(res.Ops, Op{Action: "help", Value: "true", Type: CmdGlobal})
			return 0, nil
		}
		return 0, fmt.Errorf("unknown flag: --%s", key)
	}

	consumed := 0

	switch def.ValueType {
	case ValueNone:
		if hasEq {
			return 0, fmt.Errorf("flag --%s does not take a value", key)
		}
		val = "true"
	case ValueOptional:
		if hasEq {
			// val is already set
		} else if idx+1 < len(args) && !strings.HasPrefix(args[idx+1], "-") {
			// Look ahead: if next arg is NOT a flag, consume it
			val = args[idx+1]
			consumed = 1
		} else {
			val = "true"
		}
	default: // ValueAny, ValueNumber
		if !hasEq {
			if idx+1 >= len(args) {
				return 0, fmt.Errorf("flag --%s requires a value", key)
			}
			val = args[idx+1]
			consumed = 1
		}
	}

	return consumed, addOp(res, def, val, false)
}

func parseShort(arg string, args []string, idx int, defs map[rune]CommandDef, res *ParsedArgs) (int, error) {
	chars := []rune(arg[1:])
	consumed := 0

	for j, char := range chars {
		def, ok := defs[char]
		if !ok {
			return 0, fmt.Errorf("unknown short flag: -%c", char)
		}

		// Handle flags that don't take arguments
		if def.ValueType == ValueNone {
			if err := addOp(res, def, "true", true); err != nil {
				return 0, err
			}
			continue
		}

		// Handle Optional Values (-v)
		if def.ValueType == ValueOptional {
			// Check if we are "stacking" the same char, e.g. -vv
			isStacking := false
			if j+1 < len(chars) && chars[j+1] == char {
				isStacking = true
			}

			if isStacking {
				// Treat current char as bool, continue loop to handle next 'v'
				if err := addOp(res, def, "true", true); err != nil {
					return 0, err
				}
				continue
			}

			// If data remains attached (e.g. -vdebug), use it
			if j+1 < len(chars) {
				val := string(chars[j+1:])
				if err := addOp(res, def, val, true); err != nil {
					return 0, err
				}
				break // consumed remainder
			}

			if err := addOp(res, def, "true", true); err != nil {
				return 0, err
			}
			continue
		}

		// Handle Mandatory Values
		val := ""
		if j+1 < len(chars) {
			val = string(chars[j+1:])
		} else {
			if idx+1 >= len(args) {
				return 0, fmt.Errorf("flag -%c requires a value", char)
			}
			val = args[idx+1]
			consumed = 1
		}

		if err := addOp(res, def, val, true); err != nil {
			return 0, err
		}
		break
	}
	return consumed, nil
}

func addOp(res *ParsedArgs, def CommandDef, val string, isShort bool) error {
	if def.ValueType == ValueNumber {
		if _, err := strconv.Atoi(val); err != nil {
			prefix := "--"
			if len(def.Name) == 1 {
				prefix = "-"
			}
			return fmt.Errorf("flag %s%s expects a number, got %q", prefix, def.Name, val)
		}
	}

	if def.Type == CmdGlobal {
		res.Globals[def.Name] = val
	}

	res.Ops = append(res.Ops, Op{Action: def.Name, Value: val, Type: def.Type, IsShort: isShort})
	return nil
}
