package mcp

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image/png"
	"math"
	"sort"

	"github.com/fogleman/gg"
	"github.com/mark3labs/mcp-go/mcp"
)

// AMM math tools — closed-form constant-product (Uniswap V2-style)
// calculations. These don't use the ODE machinery because the underlying
// math is simple algebra:
//
//   Spot price       = y / x
//   Output for dx in = y · dx · (1 − fee) / (x + dx · (1 − fee))
//   Slippage         = 1 − effective / spot
//   IL(r)            = 2·√r / (1 + r) − 1   where r = P_new / P_old
//
// All three tools accept reserves directly so they're usable without first
// designing a model. Pairs naturally with petri_template (load AMM model)
// → petri_visualize → petri_amm_il / petri_amm_depth for the quantitative
// view.

// ------- petri_amm_quote -------

func ammQuoteTool() mcp.Tool {
	return mcp.NewTool("petri_amm_quote",
		mcp.WithDescription("Single-trade math for a Uniswap V2-style constant-product AMM. Given reserves and a trade size, returns the output amount, effective price, spot price, slippage, and fee paid. Pure algebra — no model required."),
		mcp.WithNumber("reserve_x",
			mcp.Required(),
			mcp.Description("Reserve of token X (input token)"),
		),
		mcp.WithNumber("reserve_y",
			mcp.Required(),
			mcp.Description("Reserve of token Y (output token)"),
		),
		mcp.WithNumber("amount_in",
			mcp.Required(),
			mcp.Description("Amount of token X being swapped in"),
		),
		mcp.WithNumber("fee_bps",
			mcp.Description("Pool fee in basis points (default 30 = 0.3%, Uniswap V2 standard)"),
		),
	)
}

type ammQuoteResponse struct {
	ReserveX        float64 `json:"reserveX"`
	ReserveY        float64 `json:"reserveY"`
	AmountIn        float64 `json:"amountIn"`
	FeeBps          float64 `json:"feeBps"`
	AmountOut       float64 `json:"amountOut"`
	FeePaid         float64 `json:"feePaid"`
	SpotPrice       float64 `json:"spotPrice"`
	EffectivePrice  float64 `json:"effectivePrice"`
	SlippageBps     float64 `json:"slippageBps"`
	PriceImpactPct  float64 `json:"priceImpactPct"`
	NewReserveX     float64 `json:"newReserveX"`
	NewReserveY     float64 `json:"newReserveY"`
}

func handleAmmQuote(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	rx := request.GetFloat("reserve_x", 0)
	ry := request.GetFloat("reserve_y", 0)
	dx := request.GetFloat("amount_in", 0)
	feeBps := request.GetFloat("fee_bps", 30)
	if rx <= 0 || ry <= 0 {
		return mcp.NewToolResultError("reserve_x and reserve_y must be positive"), nil
	}
	if dx <= 0 {
		return mcp.NewToolResultError("amount_in must be positive"), nil
	}
	if feeBps < 0 || feeBps >= 10000 {
		return mcp.NewToolResultError("fee_bps must be in [0, 10000)"), nil
	}

	feeRate := feeBps / 10000.0
	dxAfterFee := dx * (1 - feeRate)
	feePaid := dx * feeRate

	// Constant product: (x + dxAfterFee) * (y - dy) = x * y
	// → dy = y * dxAfterFee / (x + dxAfterFee)
	dy := ry * dxAfterFee / (rx + dxAfterFee)
	spotPrice := ry / rx
	effectivePrice := dy / dx
	priceImpact := 1 - effectivePrice/spotPrice
	if priceImpact < 0 {
		priceImpact = 0
	}

	resp := ammQuoteResponse{
		ReserveX:        rx,
		ReserveY:        ry,
		AmountIn:        dx,
		FeeBps:          feeBps,
		AmountOut:       dy,
		FeePaid:         feePaid,
		SpotPrice:       spotPrice,
		EffectivePrice:  effectivePrice,
		SlippageBps:     priceImpact * 10000,
		PriceImpactPct:  priceImpact * 100,
		NewReserveX:     rx + dx,
		NewReserveY:     ry - dy,
	}
	text, _ := json.MarshalIndent(resp, "", "  ")
	return mcp.NewToolResultText(string(text)), nil
}

// ------- petri_amm_il -------

func ammILTool() mcp.Tool {
	return mcp.NewTool("petri_amm_il",
		mcp.WithDescription("Impermanent loss curve for a Uniswap V2-style LP. Plots IL(r) = 2·√r/(1+r) − 1 over a range of price ratios r = P_new/P_old. Optionally overlays a 'breakeven' line for a given fee APY to show where fees compensate for IL."),
		mcp.WithString("price_ratios",
			mcp.Description(`JSON array of price ratios to evaluate (e.g. [0.25, 0.5, 1, 2, 4]). Alternative to 'range'`),
		),
		mcp.WithString("range",
			mcp.Description(`JSON array [start, stop, n] generating n log-spaced ratios from start to stop. Default: [0.25, 4, 80] covers a 16x range each direction`),
		),
		mcp.WithNumber("fee_apy",
			mcp.Description("Fee APY (as decimal, e.g. 0.20 = 20%). If supplied, draws a breakeven horizontal at this level"),
		),
		mcp.WithNumber("holding_period_days",
			mcp.Description("Holding period in days (default 365). Used with fee_apy to compute realized fee return"),
		),
	)
}

type ammILResponse struct {
	PriceRatios     []float64 `json:"priceRatios"`
	IL              []float64 `json:"impermanentLossPct"`
	FeeAPY          float64   `json:"feeApy,omitempty"`
	FeeYieldPct     float64   `json:"feeYieldPct,omitempty"`
	BreakevenRatios []float64 `json:"breakevenRatios,omitempty"`
}

func handleAmmIL(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var ratios []float64
	if s := request.GetString("price_ratios", ""); s != "" {
		if err := json.Unmarshal([]byte(s), &ratios); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid price_ratios JSON: %v", err)), nil
		}
	} else {
		rng := [3]float64{0.25, 4, 80}
		if s := request.GetString("range", ""); s != "" {
			if err := json.Unmarshal([]byte(s), &rng); err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("invalid range JSON: %v", err)), nil
			}
		}
		n := int(rng[2])
		if n < 2 {
			n = 2
		}
		// Log-spaced ratios so the curve has uniform density across orders
		// of magnitude — the math is symmetric around r=1 in log space.
		ratios = logspace(rng[0], rng[1], n)
	}
	if len(ratios) == 0 {
		return mcp.NewToolResultError("need at least one price ratio"), nil
	}
	for _, r := range ratios {
		if r <= 0 {
			return mcp.NewToolResultError("price ratios must be positive"), nil
		}
	}
	sort.Float64s(ratios)

	il := make([]float64, len(ratios))
	for i, r := range ratios {
		// IL(r) = 2·√r / (1 + r) − 1
		il[i] = 100 * (2*math.Sqrt(r)/(1+r) - 1)
	}

	feeAPY := request.GetFloat("fee_apy", 0)
	days := request.GetFloat("holding_period_days", 365)
	if days <= 0 {
		days = 365
	}
	resp := ammILResponse{
		PriceRatios: ratios,
		IL:          il,
	}
	if feeAPY > 0 {
		realizedFee := feeAPY * (days / 365.0) * 100
		resp.FeeAPY = feeAPY
		resp.FeeYieldPct = realizedFee
		// Find ratios where IL ≤ −realizedFee (i.e., fee covers IL).
		// Reported as the two crossing points (above and below r=1).
		for i := 1; i < len(ratios); i++ {
			if (il[i-1]+realizedFee)*(il[i]+realizedFee) < 0 {
				// Linear interp on log-ratio.
				lr1, lr2 := math.Log(ratios[i-1]), math.Log(ratios[i])
				v1, v2 := il[i-1]+realizedFee, il[i]+realizedFee
				lr := lr1 + (lr2-lr1)*(-v1)/(v2-v1)
				resp.BreakevenRatios = append(resp.BreakevenRatios, math.Exp(lr))
			}
		}
	}

	text, _ := json.MarshalIndent(resp, "", "  ")
	if pngBytes, perr := renderILPNG(resp); perr == nil {
		return mcp.NewToolResultImage(string(text), base64.StdEncoding.EncodeToString(pngBytes), "image/png"), nil
	}
	return mcp.NewToolResultText(string(text)), nil
}

// ------- petri_amm_depth -------

func ammDepthTool() mcp.Tool {
	return mcp.NewTool("petri_amm_depth",
		mcp.WithDescription("Depth chart for a constant-product AMM: plots slippage (or output price) as a function of trade size. Use to answer 'how big can I trade before paying X% slippage?' and to size positions against pool depth."),
		mcp.WithNumber("reserve_x",
			mcp.Required(),
			mcp.Description("Reserve of token X (input)"),
		),
		mcp.WithNumber("reserve_y",
			mcp.Required(),
			mcp.Description("Reserve of token Y (output)"),
		),
		mcp.WithNumber("fee_bps",
			mcp.Description("Pool fee in basis points (default 30)"),
		),
		mcp.WithString("trade_sizes",
			mcp.Description("JSON array of trade sizes (in X). Alternative to size_range"),
		),
		mcp.WithString("size_range",
			mcp.Description("JSON array [min_pct, max_pct, n] where pcts are fractions of reserve_x. Default [0.001, 0.5, 80]"),
		),
	)
}

type depthPoint struct {
	TradeSize      float64 `json:"tradeSize"`
	AmountOut      float64 `json:"amountOut"`
	EffectivePrice float64 `json:"effectivePrice"`
	SlippageBps    float64 `json:"slippageBps"`
}

type ammDepthResponse struct {
	ReserveX  float64      `json:"reserveX"`
	ReserveY  float64      `json:"reserveY"`
	FeeBps    float64      `json:"feeBps"`
	SpotPrice float64      `json:"spotPrice"`
	Points    []depthPoint `json:"points"`
}

func handleAmmDepth(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	rx := request.GetFloat("reserve_x", 0)
	ry := request.GetFloat("reserve_y", 0)
	if rx <= 0 || ry <= 0 {
		return mcp.NewToolResultError("reserves must be positive"), nil
	}
	feeBps := request.GetFloat("fee_bps", 30)
	feeRate := feeBps / 10000.0

	var sizes []float64
	if s := request.GetString("trade_sizes", ""); s != "" {
		if err := json.Unmarshal([]byte(s), &sizes); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid trade_sizes JSON: %v", err)), nil
		}
	} else {
		rng := [3]float64{0.001, 0.5, 80}
		if s := request.GetString("size_range", ""); s != "" {
			if err := json.Unmarshal([]byte(s), &rng); err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("invalid size_range JSON: %v", err)), nil
			}
		}
		n := int(rng[2])
		if n < 2 {
			n = 2
		}
		sizes = logspace(rng[0]*rx, rng[1]*rx, n)
	}
	sort.Float64s(sizes)

	spotPrice := ry / rx
	points := make([]depthPoint, 0, len(sizes))
	for _, dx := range sizes {
		if dx <= 0 {
			continue
		}
		dxAfterFee := dx * (1 - feeRate)
		dy := ry * dxAfterFee / (rx + dxAfterFee)
		eff := dy / dx
		slip := (1 - eff/spotPrice) * 10000
		if slip < 0 {
			slip = 0
		}
		points = append(points, depthPoint{TradeSize: dx, AmountOut: dy, EffectivePrice: eff, SlippageBps: slip})
	}

	resp := ammDepthResponse{
		ReserveX:  rx,
		ReserveY:  ry,
		FeeBps:    feeBps,
		SpotPrice: spotPrice,
		Points:    points,
	}
	text, _ := json.MarshalIndent(resp, "", "  ")
	if pngBytes, perr := renderDepthPNG(resp); perr == nil {
		return mcp.NewToolResultImage(string(text), base64.StdEncoding.EncodeToString(pngBytes), "image/png"), nil
	}
	return mcp.NewToolResultText(string(text)), nil
}

// ------- helpers -------

func logspace(start, stop float64, n int) []float64 {
	if start <= 0 || stop <= 0 {
		return linspace(start, stop, n)
	}
	if n <= 1 {
		return []float64{start}
	}
	ls := math.Log(start)
	le := math.Log(stop)
	out := make([]float64, n)
	for i := 0; i < n; i++ {
		out[i] = math.Exp(ls + (le-ls)*float64(i)/float64(n-1))
	}
	return out
}

// ------- renderers -------

func renderILPNG(resp ammILResponse) ([]byte, error) {
	const W, H = 720, 420
	dc := gg.NewContext(W, H)
	dc.SetHexColor("#ffffff")
	dc.Clear()

	if len(resp.PriceRatios) == 0 {
		return nil, fmt.Errorf("no data")
	}
	// X-axis log-scale visually — we plot vs log(r) but label as r.
	lrs := make([]float64, len(resp.PriceRatios))
	for i, r := range resp.PriceRatios {
		lrs[i] = math.Log(r)
	}
	xmin, xmax := lrs[0], lrs[len(lrs)-1]
	ymin := math.Inf(1)
	ymax := 0.0
	for _, v := range resp.IL {
		if v < ymin {
			ymin = v
		}
		if v > ymax {
			ymax = v
		}
	}
	if resp.FeeYieldPct > 0 {
		// Make sure the breakeven line is visible.
		if -resp.FeeYieldPct < ymin {
			ymin = -resp.FeeYieldPct * 1.1
		}
	}
	yrange := ymax - ymin
	if yrange < 1 {
		yrange = 1
	}
	ymin -= yrange * 0.1
	ymax += yrange * 0.1

	title := "Impermanent Loss vs Price Ratio"
	drawPlotFrame(dc, xmin, xmax, ymin, ymax, title, "Price Ratio (log)", "IL (%)", 0, 0, W, H)

	const (
		marginT = 40.0
		marginR = 140.0
		marginB = 50.0
		marginL = 70.0
	)
	plotW := float64(W) - marginL - marginR
	plotH := float64(H) - marginT - marginB
	sx := func(lr float64) float64 {
		return marginL + (lr-xmin)/(xmax-xmin)*plotW
	}
	sy := func(y float64) float64 {
		return marginT + plotH - (y-ymin)/(ymax-ymin)*plotH
	}

	// IL curve.
	dc.SetHexColor("#d32f2f")
	dc.SetLineWidth(2)
	dc.MoveTo(sx(lrs[0]), sy(resp.IL[0]))
	for i := 1; i < len(lrs); i++ {
		dc.LineTo(sx(lrs[i]), sy(resp.IL[i]))
	}
	dc.Stroke()

	// Fee yield horizontal (if supplied).
	if resp.FeeYieldPct > 0 {
		dc.SetHexColor("#1976d2")
		dc.SetLineWidth(1.5)
		dc.SetDash(6, 4)
		dc.MoveTo(sx(xmin), sy(-resp.FeeYieldPct))
		dc.LineTo(sx(xmax), sy(-resp.FeeYieldPct))
		dc.Stroke()
		dc.SetDash()
		for _, r := range resp.BreakevenRatios {
			dc.SetHexColor("#1976d2")
			dc.DrawCircle(sx(math.Log(r)), sy(-resp.FeeYieldPct), 5)
			dc.Fill()
			dc.SetHexColor("#ffffff")
			dc.SetLineWidth(1)
			dc.DrawCircle(sx(math.Log(r)), sy(-resp.FeeYieldPct), 5)
			dc.Stroke()
		}
	}

	// Mark r=1 as vertical reference.
	if 0 >= xmin && 0 <= xmax {
		dc.SetHexColor("#999999")
		dc.SetLineWidth(1)
		dc.SetDash(3, 3)
		dc.MoveTo(sx(0), sy(ymin))
		dc.LineTo(sx(0), sy(ymax))
		dc.Stroke()
		dc.SetDash()
	}

	// Legend.
	legendX := marginL + plotW + 14
	legendY := marginT + 6
	if f, err := pngFace(false, 11); err == nil {
		dc.SetFontFace(f)
		dc.SetHexColor("#d32f2f")
		dc.SetLineWidth(2)
		dc.DrawLine(legendX, legendY+6, legendX+20, legendY+6)
		dc.Stroke()
		dc.SetHexColor("#000000")
		dc.DrawStringAnchored("IL", legendX+24, legendY+6, 0, 0.5)
		legendY += 18
		if resp.FeeYieldPct > 0 {
			dc.SetHexColor("#1976d2")
			dc.SetDash(4, 3)
			dc.DrawLine(legendX, legendY+6, legendX+20, legendY+6)
			dc.Stroke()
			dc.SetDash()
			dc.SetHexColor("#000000")
			dc.DrawStringAnchored(fmt.Sprintf("fee yield (%.1f%%)", resp.FeeYieldPct), legendX+24, legendY+6, 0, 0.5)
			legendY += 18
			if len(resp.BreakevenRatios) > 0 {
				legendY += 4
				dc.SetHexColor("#666666")
				dc.DrawStringAnchored("breakeven:", legendX, legendY+6, 0, 0.5)
				legendY += 14
				for _, r := range resp.BreakevenRatios {
					dc.DrawStringAnchored(fmt.Sprintf("  r=%.3g", r), legendX, legendY+6, 0, 0.5)
					legendY += 14
				}
			}
		}
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, dc.Image()); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func renderDepthPNG(resp ammDepthResponse) ([]byte, error) {
	const W, H = 720, 420
	dc := gg.NewContext(W, H)
	dc.SetHexColor("#ffffff")
	dc.Clear()

	if len(resp.Points) == 0 {
		return nil, fmt.Errorf("no data")
	}
	xs := make([]float64, len(resp.Points))
	slippage := make([]float64, len(resp.Points))
	for i, p := range resp.Points {
		xs[i] = p.TradeSize / resp.ReserveX * 100 // x-axis: trade as % of pool
		slippage[i] = p.SlippageBps / 100         // y-axis: slippage in %
	}

	title := fmt.Sprintf("AMM Depth — pool (%.4g, %.4g), fee %.0fbps", resp.ReserveX, resp.ReserveY, resp.FeeBps)
	drawXYPlot(dc, xs, [][]float64{slippage}, []string{"slippage"}, title, "Trade size (% of pool)", "Slippage (%)", 0, 0, W, H)

	var buf bytes.Buffer
	if err := png.Encode(&buf, dc.Image()); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
