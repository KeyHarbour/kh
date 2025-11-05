package main

// version is injected at build time using -ldflags "-X main.version=..."
// It defaults to 'dev' for local builds.
var version = "dev"
