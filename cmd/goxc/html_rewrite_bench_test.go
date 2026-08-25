package main

import (
	"fmt"
	"strings"
	"testing"
)

var benchmarkRawElementTag htmlTag
var benchmarkScannedHTML scannedHTML
var benchmarkHTMLRewriteOutput string

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

func BenchmarkCustomIndexHTMLContext(b *testing.B) {
	for _, count := range []int{1_000, 10_000, 50_000} {
		for _, test := range []struct {
			name    string
			content func(int) string
		}{
			{
				name: "matched",
				content: func(count int) string {
					return strings.Repeat("<div>", count) + strings.Repeat("</div>", count)
				},
			},
			{
				name: "unmatched",
				content: func(count int) string {
					return strings.Repeat("<div>", count) + strings.Repeat("</span>", count)
				},
			},
			{
				name: "mixed-repeated-names",
				content: func(count int) string {
					return strings.Repeat("<div><section>", count) + strings.Repeat("</div></section>", count)
				},
			},
		} {
			b.Run(fmt.Sprintf("%s/%d", test.name, count), func(b *testing.B) {
				content := test.content(count)
				b.SetBytes(int64(len(content)))
				b.ReportAllocs()
				b.ResetTimer()
				for range b.N {
					document, err := scanCustomIndexHTML(content)
					if err != nil {
						b.Fatal(err)
					}
					benchmarkScannedHTML = document
				}
			})
		}
	}
}

func BenchmarkCustomIndexRewritePlan(b *testing.B) {
	for _, count := range []int{10, 100, 1_000, 10_000} {
		for _, growing := range []bool{false, true} {
			name := "fixed"
			documentBytes := 1 << 20
			if growing {
				name = "growing"
				documentBytes = count * 32
			}
			b.Run(fmt.Sprintf("%s/%d", name, count), func(b *testing.B) {
				content := strings.Repeat("x", documentBytes)
				replacements := make([]htmlReplacement, 0, count)
				for index := range count {
					start := index * documentBytes / count
					replacements = append(replacements, htmlReplacement{
						start: start, end: start + 1, value: "replacement", description: "benchmark",
					})
				}
				plan := htmlRewritePlan{content: content, replacements: replacements}
				b.SetBytes(int64(documentBytes))
				b.ReportAllocs()
				b.ResetTimer()
				for range b.N {
					result, err := plan.apply()
					if err != nil {
						b.Fatal(err)
					}
					benchmarkHTMLRewriteOutput = result
				}
			})
		}
	}
}
