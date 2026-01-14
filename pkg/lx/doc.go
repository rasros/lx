// Package lx provides a library for discovering, slicing, and formatting file system content
// specifically for Large Language Model (LLM) contexts.
//
// It replicates the core logic of the 'lx' CLI tool, allowing developers to embed
// file walking, gitignore processing, token estimation, and Markdown/XML rendering
// into their own applications.
//
// The core workflow involves two main components:
//
// 1. Walker: Recursively discovers files while respecting .gitignore, .ignore, and .lxignore files.
//
// 2. Runner: Processes individual files, applies slicing (head/tail/lines), and renders
// them using customizable templates.
//
// Basic Usage:
//
//	tmplEngine, cfg, _ := lx.Options{Head: nil}.CompileTemplates()
//	walker := lx.NewWalker(*cfg)
//	// ... iterate over walker.Walk() ...
package lx
