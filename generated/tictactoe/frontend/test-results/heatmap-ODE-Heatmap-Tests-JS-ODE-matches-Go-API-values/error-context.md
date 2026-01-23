# Page snapshot

```yaml
- generic [ref=e1]:
  - heading "ODE Heatmap Tests" [level=1] [ref=e2]
  - button "Run Tests" [ref=e3] [cursor=pointer]
  - button "Compare JS vs API" [active] [ref=e4] [cursor=pointer]
  - generic [ref=e5]: "=== Comparing JS ODE vs Go API === JS ODE: 8639ms, Total: 8654ms Position | JS ODE | Go API | Diff ---------|---------|---------|------- Error: Cannot read properties of undefined (reading '00') Make sure the Go backend is running (go run ./cmd/petri-pilot serve tic-tac-toe)"
```