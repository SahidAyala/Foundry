package session_test

import (
	"testing"

	"foundry/session"
)

func TestParseLine_SlashCommandWithArgs(t *testing.T) {
	cmd, isSlash := session.ParseLine(`/feature implementa refresh tokens con JWT`)
	if !isSlash {
		t.Fatal("isSlash = false, want true")
	}
	if cmd.Name != "feature" {
		t.Errorf("Name = %q, want %q", cmd.Name, "feature")
	}
	if cmd.Args != "implementa refresh tokens con JWT" {
		t.Errorf("Args = %q, want %q", cmd.Args, "implementa refresh tokens con JWT")
	}
}

func TestParseLine_SlashCommandWithoutArgs(t *testing.T) {
	cmd, isSlash := session.ParseLine(`/init`)
	if !isSlash {
		t.Fatal("isSlash = false, want true")
	}
	if cmd.Name != "init" {
		t.Errorf("Name = %q, want %q", cmd.Name, "init")
	}
	if cmd.Args != "" {
		t.Errorf("Args = %q, want empty", cmd.Args)
	}
}

func TestParseLine_NameIsLowercased(t *testing.T) {
	cmd, isSlash := session.ParseLine(`/FEATURE do X`)
	if !isSlash {
		t.Fatal("isSlash = false, want true")
	}
	if cmd.Name != "feature" {
		t.Errorf("Name = %q, want %q", cmd.Name, "feature")
	}
}

func TestParseLine_TrimsSurroundingWhitespace(t *testing.T) {
	cmd, isSlash := session.ParseLine("  /feature   do X   \n")
	if !isSlash {
		t.Fatal("isSlash = false, want true")
	}
	if cmd.Name != "feature" {
		t.Errorf("Name = %q, want %q", cmd.Name, "feature")
	}
	if cmd.Args != "do X" {
		t.Errorf("Args = %q, want %q", cmd.Args, "do X")
	}
}

func TestParseLine_PlainTextIsNotASlashCommand(t *testing.T) {
	_, isSlash := session.ParseLine(`implementa refresh tokens con JWT`)
	if isSlash {
		t.Error("isSlash = true for plain text, want false")
	}
}

func TestParseLine_EmptyLineIsNotASlashCommand(t *testing.T) {
	_, isSlash := session.ParseLine("   ")
	if isSlash {
		t.Error("isSlash = true for a blank line, want false")
	}
}
