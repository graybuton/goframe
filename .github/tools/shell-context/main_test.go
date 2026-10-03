package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestExecutableContext(t *testing.T) {
	src := []byte("printf '%s\\n' 'bash'\ncat <<'EOF'\nbash\nEOF\n" +
		"printf x |\nbash\nif sh; then :; fi\n" +
		"cat <<EOF\n$(bash)\nEOF\n")
	events, err := analyze(src, "fixture.sh")
	if err != nil {
		t.Fatal(err)
	}
	var commands []string
	for _, event := range events {
		commands = append(commands, event.command)
	}
	joined := strings.Join(commands, "|")
	if len(events) != 8 || strings.Count(joined, "bash") != 3 ||
		commands[1] != "cat" || commands[3] != "bash" ||
		commands[4] != "sh" || commands[6] != "cat" || commands[7] != "bash" {
		t.Fatalf("unexpected executable calls: %q", commands)
	}
	if !events[3].pipeInput {
		t.Fatalf("pipeline receiver lost stdin context: %+v", events[3])
	}
	var output bytes.Buffer
	if err := writeEvents(&output, events); err != nil {
		t.Fatal(err)
	}
	if !bytes.HasSuffix(output.Bytes(), []byte("E\x008\x00")) {
		t.Fatalf("missing complete event trailer: %q", output.Bytes())
	}
}

func TestInvalidSourceFailsClosed(t *testing.T) {
	for _, src := range []string{"if then", "bash\x00"} {
		if _, err := analyze([]byte(src), "invalid.sh"); err == nil {
			t.Fatalf("accepted invalid source %q", src)
		}
	}
}

func TestNestedCallIsNotDoubleCountedAsOuterProvenance(t *testing.T) {
	events, err := analyze([]byte("result=\"$(curl https://example.invalid/file)\"\n"), "fixture.sh")
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 || strings.Contains(events[0].command, "curl") ||
		!strings.Contains(events[1].command, "curl") {
		t.Fatalf("nested command provenance is not isolated: %+v", events)
	}
}

func TestLiteralDollarQuotedWordsRetainCommandIdentity(t *testing.T) {
	events, err := analyze([]byte("$'bash'\nba$'sh'\nprintf '%s\\n' $'bash'\n"), "fixture.sh")
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 3 || events[0].words[0] != "'bash'" ||
		events[1].words[0] != "ba'sh'" || events[2].words[2] != "'bash'" {
		t.Fatalf("literal ANSI-C quoting changed command identity: %+v", events)
	}
}

func TestLineContinuationRetainsCommandIdentity(t *testing.T) {
	events, err := analyze([]byte("ba\\\nsh\nprintf '%s\\n' ba\\\nsh\n"), "fixture.sh")
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 || events[0].words[0] != "bash" || events[1].words[2] != "bash" {
		t.Fatalf("line continuation changed command identity: %+v", events)
	}
}

func TestQuotedLineContinuationFollowsShellContext(t *testing.T) {
	events, err := analyze([]byte("\"ba\\\nsh\"\nprintf '%s\\n' 'ba\\\nsh'\n"), "fixture.sh")
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 || events[0].words[0] != "\"bash\"" ||
		events[1].words[2] != "'ba\\\nsh'" {
		t.Fatalf("quoted line continuation lost shell context: %+v", events)
	}
}
