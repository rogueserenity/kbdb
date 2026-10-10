// Package keyboardparts assigns ids to a keyboard's plates and PCBs on a
// write. A keyboard write replaces its whole part lists, so a part keeps its
// id - and the builds referencing it - only when the request sends that id
// back. Shared by REST and MCP so both apply the same rule.
package keyboardparts
