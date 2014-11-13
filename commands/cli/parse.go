package cli

import (
	"errors"
	"fmt"
	"os"
	"strings"

	cmds "github.com/jbenet/go-ipfs/commands"
)

// ErrInvalidSubcmd signals when the parse error is not found
var ErrInvalidSubcmd = errors.New("subcommand not found")

// Parse parses the input commandline string (cmd, flags, and args).
// returns the corresponding command Request object.
// Parse will search each root to find the one that best matches the requested subcommand.
// TODO: get rid of extraneous return values (e.g. we ended up not needing the root value anymore)
// TODO: get rid of multiple-root support, we should only need one now
func Parse(input []string, root *cmds.Command) (cmds.Request, *cmds.Command, *cmds.Command, []string, error) {
	// use the root that matches the longest path (most accurately matches request)
	path, input, cmd := parsePath(input, root)
	opts, stringArgs, err := parseOptions(input)
	if err != nil {
		return nil, root, cmd, path, err
	}

	if len(path) == 0 {
		return nil, root, nil, path, ErrInvalidSubcmd
	}

	args, err := parseArgs(stringArgs, cmd)
	if err != nil {
		return nil, root, cmd, path, err
	}

	optDefs, err := root.GetOptions(path)
	if err != nil {
		return nil, root, cmd, path, err
	}

	req := cmds.NewRequest(path, opts, args, cmd, optDefs)

	err = cmd.CheckArguments(req)
	if err != nil {
		return req, root, cmd, path, err
	}

	return req, root, cmd, path, nil
}

// parsePath separates the command path and the opts and args from a command string
// returns command path slice, rest slice, and the corresponding *cmd.Command
func parsePath(input []string, root *cmds.Command) ([]string, []string, *cmds.Command) {
	cmd := root
	i := 0

	for _, blob := range input {
		if strings.HasPrefix(blob, "-") {
			break
		}

		sub := cmd.Subcommand(blob)
		if sub == nil {
			break
		}
		cmd = sub

		i++
	}

	return input[:i], input[i:], cmd
}

// parseOptions parses the raw string values of the given options
// returns the parsed options as strings, along with the CLI args
func parseOptions(input []string) (map[string]interface{}, []string, error) {
	opts := make(map[string]interface{})
	args := []string{}

	for i := 0; i < len(input); i++ {
		blob := input[i]

		if strings.HasPrefix(blob, "-") {
			name := blob[1:]
			value := ""

			// support single and double dash
			if strings.HasPrefix(name, "-") {
				name = name[1:]
			}

			if strings.Contains(name, "=") {
				split := strings.SplitN(name, "=", 2)
				name = split[0]
				value = split[1]
			}

			if _, ok := opts[name]; ok {
				return nil, nil, fmt.Errorf("Duplicate values for option '%s'", name)
			}

			opts[name] = value

		} else {
			args = append(args, blob)
		}
	}

	return opts, args, nil
}

func parseArgs(stringArgs []string, cmd *cmds.Command) ([]interface{}, error) {
	args := make([]interface{}, 0)

	// count required argument definitions
	lenRequired := 0
	for _, argDef := range cmd.Arguments {
		if argDef.Required {
			lenRequired++
		}
	}

	valueIndex := 0 // the index of the current stringArgs value
	for _, argDef := range cmd.Arguments {
		// skip optional argument definitions if there aren't sufficient remaining values
		if len(stringArgs)-valueIndex <= lenRequired && !argDef.Required {
			continue
		} else if argDef.Required {
			lenRequired--
		}

		if valueIndex >= len(stringArgs) {
			break
		}

		if argDef.Variadic {
			for _, arg := range stringArgs[valueIndex:] {
				var err error
				args, err = appendArg(args, argDef, arg)
				if err != nil {
					return nil, err
				}
				valueIndex++
			}
		} else {
			var err error
			args, err = appendArg(args, argDef, stringArgs[valueIndex])
			if err != nil {
				return nil, err
			}
			valueIndex++
		}
	}

	if len(stringArgs)-valueIndex > 0 {
		args = append(args, make([]interface{}, len(stringArgs)-valueIndex))
	}

	return args, nil
}

func appendArg(args []interface{}, argDef cmds.Argument, value string) ([]interface{}, error) {
	if argDef.Type == cmds.ArgString {
		return append(args, value), nil

	} else {
		in, err := os.Open(value) // FIXME(btc) must close file. fix before merge
		if err != nil {
			return nil, err
		}
		return append(args, in), nil
	}
}
