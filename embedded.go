package main

import "embed"

//go:embed lib/jagex_linker.py
//go:embed lib/jagex_linker_mailtm.py
var FS embed.FS
