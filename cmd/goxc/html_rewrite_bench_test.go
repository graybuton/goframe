package main

import (
	"fmt"
	"strings"
	"testing"
)

var benchmarkRawElementTag htmlTag

func BenchmarkCustomIndexRawElementScanning(b *testing.B) {
	for _, size := range []int{64 << 10, 1 << 20, 8 << 20} {
		b.Run(fmt.Sprintf("style/%dKiB", size>>10), func(b *testing.B) {
			content := strings.Repeat("x", size) + "</style>"
			b.SetBytes(int64(len(content)))
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				tag, err := scanRawElementClose(content, 0, "style")
				if err != nil {
					b.Fatal(err)
				}
				benchmarkRawElementTag = tag
			}
		})
		b.Run(fmt.Sprintf("script/%dKiB", size>>10), func(b *testing.B) {
			content := strings.Repeat("x", size) + "</script>"
			b.SetBytes(int64(len(content)))
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				tag, err := scanScriptElementClose(content, 0)
				if err != nil {
					b.Fatal(err)
				}
				benchmarkRawElementTag = tag
			}
		})
		b.Run(fmt.Sprintf("script-double-escaped/%dKiB", size>>10), func(b *testing.B) {
			content := "<!--<script>" + strings.Repeat("x", size) + "</script>--></script>"
			b.SetBytes(int64(len(content)))
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				tag, err := scanScriptElementClose(content, 0)
				if err != nil {
					b.Fatal(err)
				}
				benchmarkRawElementTag = tag
			}
		})
	}
}
