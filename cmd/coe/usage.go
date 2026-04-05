package main

import "fmt"

func printUsage() {
	fmt.Println("coe - Coe dictation assistant for Linux desktops")
	fmt.Println()
	fmt.Println("usage:")
	fmt.Println("  coe doctor")
	fmt.Println("  coe config init")
	fmt.Println("  coe config set <key> <value>")
	fmt.Println("  coe hotkey pick")
	fmt.Println("  coe restart")
	fmt.Println("  coe serve [--log-level <debug|info|warn|error>]")
	fmt.Println("  coe trigger <toggle|start|stop|status>")
	fmt.Println("  coe version")
}

func printVersion() {
	fmt.Printf("coe %s\n", version)
	fmt.Printf("commit: %s\n", commit)
	fmt.Printf("date: %s\n", date)
	fmt.Printf("built_by: %s\n", builtBy)
}
