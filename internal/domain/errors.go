// Package domain holds closed vocabularies shared by every boundary.
package domain

import (
	"fmt"
	"strings"
)

// ErrorCode is the closed refusal vocabulary in rhino.area.reason form.
type ErrorCode int

const (
	ArgsUnrecognized ErrorCode = iota
	ArgsIncomplete
	InputUnreadable
	RepositoryUnusable
	ConfigUnusable
	ConfigUndeclared
	PathEscapesRoot
	FileMissing
	FileUnreadable
	FileExists
	FileUnwritable
	GateChildNotFound
	GateChildNotExecutable
	GateChildRefused
	GateUndeclared
	ToolchainFailed
	ToolchainUndeclared
	HarnessRefused
)

// All lists every member in --help publication order.
var All = []ErrorCode{
	ArgsUnrecognized,
	ArgsIncomplete,
	InputUnreadable,
	RepositoryUnusable,
	ConfigUnusable,
	ConfigUndeclared,
	PathEscapesRoot,
	FileMissing,
	FileUnreadable,
	FileExists,
	FileUnwritable,
	GateChildNotFound,
	GateChildNotExecutable,
	GateChildRefused,
	GateUndeclared,
	ToolchainFailed,
	ToolchainUndeclared,
	HarnessRefused,
}

// String returns the dotted code.
func (c ErrorCode) String() string {
	switch c {
	case ArgsUnrecognized:
		return "rhino.args.unrecognized"
	case ArgsIncomplete:
		return "rhino.args.incomplete"
	case InputUnreadable:
		return "rhino.input.unreadable"
	case RepositoryUnusable:
		return "rhino.repository.unusable"
	case ConfigUnusable:
		return "rhino.config.unusable"
	case ConfigUndeclared:
		return "rhino.config.undeclared"
	case PathEscapesRoot:
		return "rhino.path.escapes-root"
	case FileMissing:
		return "rhino.file.missing"
	case FileUnreadable:
		return "rhino.file.unreadable"
	case FileExists:
		return "rhino.file.exists"
	case FileUnwritable:
		return "rhino.file.unwritable"
	case GateChildNotFound:
		return "rhino.gate.child-not-found"
	case GateChildNotExecutable:
		return "rhino.gate.child-not-executable"
	case GateChildRefused:
		return "rhino.gate.child-refused"
	case GateUndeclared:
		return "rhino.gate.undeclared"
	case ToolchainFailed:
		return "rhino.toolchain.failed"
	case ToolchainUndeclared:
		return "rhino.toolchain.undeclared"
	case HarnessRefused:
		return "rhino.harness.refused"
	default:
		return "rhino.args.unrecognized"
	}
}

// Format selects the refusal rendering.
type Format int

const (
	Text Format = iota
	JSON
)

// Quote renders a JSON string for one value.
func Quote(value string) string {
	var out strings.Builder
	out.WriteByte('"')
	for _, r := range value {
		switch r {
		case '"':
			out.WriteString("\\\"")
		case '\\':
			out.WriteString("\\\\")
		case '\n':
			out.WriteString("\\n")
		case '\r':
			out.WriteString("\\r")
		case '\t':
			out.WriteString("\\t")
		default:
			if r < ' ' {
				fmt.Fprintf(&out, "\\u%04x", r)
			} else {
				out.WriteRune(r)
			}
		}
	}
	out.WriteByte('"')
	return out.String()
}

// Body renders a refusal for the requested format.
func Body(format Format, code ErrorCode, message string) string {
	message = strings.TrimRight(message, "\n")
	if format == JSON {
		return fmt.Sprintf("{\"schemaVersion\":1,\"error\":{\"code\":%s,\"message\":%s}}\n",
			Quote(code.String()), Quote(message))
	}
	return fmt.Sprintf("rhino: %s\n", message)
}
