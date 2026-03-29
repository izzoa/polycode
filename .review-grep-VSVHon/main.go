package main

import (
  "encoding/json"
  "fmt"
  "os"
  "path/filepath"

  "github.com/izzoa/polycode/internal/action"
  "github.com/izzoa/polycode/internal/provider"
)

func main() {
  wd, _ := os.Getwd()
  dataDir := filepath.Join(wd, ".review-grep-data")
  _ = os.RemoveAll(dataDir)
  _ = os.MkdirAll(dataDir, 0o755)
  defer os.RemoveAll(dataDir)

  content := "a1\nMATCH1\na3\na4\na5\na6\na7\na8\nMATCH2\na10\n"
  _ = os.WriteFile(filepath.Join(dataDir, "ctx.txt"), []byte(content), 0o644)

  orig, _ := os.Getwd()
  _ = os.Chdir(dataDir)
  defer os.Chdir(orig)

  args, _ := json.Marshal(map[string]any{"pattern": "MATCH", "context_lines": 2, "max_count": 1})
  e := action.NewExecutor(nil, 0)
  result := e.Execute(provider.ToolCall{ID: "1", Name: "grep_search", Arguments: string(args)})
  if result.Error != nil {
    fmt.Println("ERR:", result.Error)
    return
  }
  fmt.Print(result.Output)
}
