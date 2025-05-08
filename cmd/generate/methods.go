// Copyright 2025 Phil Isaac. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"fmt"
	"strings"
)

func cleanDocComment(doc string) string {
	// Remove @tags and convert to standard Go docs
	doc = strings.ReplaceAll(doc, "@since", "Since")
	doc = strings.ReplaceAll(doc, "@proposed", "PROPOSED")

	// Fix line breaks in comments and normalize whitespace
	var builder strings.Builder
	lines := strings.Split(doc, "\n")

	isFirstLine := true
	for _, line := range lines {
		line = strings.TrimSpace(line)
		// Skip empty lines
		if line == "" {
			continue
		}

		// Remove backticks
		line = strings.ReplaceAll(line, "`", "")

		// Replace problematic placeholder syntax
		line = strings.ReplaceAll(line, "{@link ", "")
		line = strings.ReplaceAll(line, "}", "")

		if !isFirstLine {
			builder.WriteString(" ") // Join all lines with space
		}
		isFirstLine = false

		builder.WriteString(line)
	}

	return builder.String()
}

func generateMethodForRequest(out *bytes.Buffer, r *Request) {
	methodName := methodName(r.Method)

	// Generate doc comment
	fmt.Fprintf(out, "\n// %s\n", methodName+" sends a "+r.Method+" request to the LSP server.")
	if r.Documentation != "" {
		for _, line := range strings.SplitN(cleanDocComment(r.Documentation), "\n", -1) {
			fmt.Fprintf(out, "// %s\n", line)
		}
	}

	var paramType, resultType string
	if notNil(r.Params) {
		paramType = goplsName(r.Params)
	}
	if notNil(r.Result) {
		resultType = goplsName(r.Result)
		if resultType == "interface{}" || resultType == "string" || resultType == "any" {
		} else if strings.HasPrefix(resultType, "*") {
			resultType = "*protocol." + resultType[1:]
		} else if strings.HasPrefix(resultType, "[]") {
			resultType = "[]protocol." + resultType[2:]
		} else {
			resultType = "protocol." + resultType
		}
	}

	// Function signature
	fmt.Fprintf(out, "func (c *Client) %s(ctx context.Context", methodName)
	if paramType != "" {
		fmt.Fprintf(out, ", params protocol.%s", paramType)
	}
	if resultType != "" {

		fmt.Fprintf(out, ") (%s, error) {\n", resultType)
	} else {
		fmt.Fprintf(out, ") error {\n")
	}

	// Function body
	if resultType != "" {
		fmt.Fprintf(out, "\tvar result %s\n", resultType)
		if paramType != "" {
			fmt.Fprintf(out, "\terr := c.Call(ctx, %q, params, &result)\n", r.Method)
		} else {
			fmt.Fprintf(out, "\terr := c.Call(ctx, %q, nil, &result)\n", r.Method)
		}
		fmt.Fprintf(out, "\treturn result, err\n")
	} else {
		if paramType != "" {
			fmt.Fprintf(out, "\treturn c.Call(ctx, %q, params, nil)\n", r.Method)
		} else {
			fmt.Fprintf(out, "\treturn c.Call(ctx, %q, nil, nil)\n", r.Method)
		}
	}
	fmt.Fprintf(out, "}\n")
}

func generateMethodForNotification(out *bytes.Buffer, n *Notification) {
	methodName := methodName(n.Method)

	// Generate doc comment
	fmt.Fprintf(out, "\n// %s\n", methodName+" sends a "+n.Method+" notification to the LSP server.")
	if n.Documentation != "" {
		for _, line := range strings.SplitN(cleanDocComment(n.Documentation), "\n", -1) {
			fmt.Fprintf(out, "// %s\n", line)
		}
	}

	var paramType string
	if notNil(n.Params) {
		paramType = goplsName(n.Params)
	}

	// Function signature
	fmt.Fprintf(out, "func (c *Client) %s(ctx context.Context", methodName)
	if paramType != "" {
		fmt.Fprintf(out, ", params protocol.%s", paramType)
	}
	fmt.Fprintf(out, ") error {\n")

	// Function body
	if paramType != "" {
		fmt.Fprintf(out, "\treturn c.Notify(ctx, %q, params)\n", n.Method)
	} else {
		fmt.Fprintf(out, "\treturn c.Notify(ctx, %q, nil)\n", n.Method)
	}
	fmt.Fprintf(out, "}\n")
}

func generateMethods(model *Model) string {
	out := new(bytes.Buffer)

	// Write header
	fmt.Fprint(out, `// Generated code. Do not edit
package lsp

import (
	"context"

  "github.com/isaacphi/mcp-language-server/internal/protocol"
)
`)

	// Generate methods for each request
	for _, r := range model.Requests {
		if r.Direction == "serverToClient" {
			continue // Skip server->client methods
		}
		generateMethodForRequest(out, r)
	}

	// Generate methods for each notification
	for _, n := range model.Notifications {
		if n.Direction == "serverToClient" {
			continue // Skip server->client notifications
		}
		if n.Method == "$/cancelRequest" {
			continue // Skip cancel request as it's handled internally by jsonrpc2
		}
		generateMethodForNotification(out, n)
	}

	return out.String()
}
