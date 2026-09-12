package server

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/brokenbots/criteriadb/pkg/memory"
	pb "github.com/brokenbots/criteriadb/pkg/pb/criteriadb/v1"
)

type GraphData struct {
	Nodes []GraphNode `json:"nodes"`
	Links []GraphLink `json:"links"`
}

type GraphNode struct {
	ID        string `json:"id"`
	Label     string `json:"label"`
	Summary   string `json:"summary"`
	Type      string `json:"type"`
	Tense     string `json:"tense"`
	Project   string `json:"project"`
	AdapterID string `json:"adapter_id"`
}

type GraphLink struct {
	Source   string  `json:"source"`
	Target   string  `json:"target"`
	Relation string  `json:"relation"`
	Weight   float64 `json:"weight"`
}

// StartWebServer starts a lightweight HTTP server serving the interactive browser visualizer.
func StartWebServer(addr string, engine *memory.MemoryEngine) error {
	mux := http.NewServeMux()

	// REST API: Graph Data endpoint
	mux.HandleFunc("/api/graph", func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		results, err := engine.Recall(ctx, &pb.QueryRequest{
			ScopeFilter: &pb.AdapterScopeFilter{
				TargetAdapterIds: []string{"*"},
			},
			Limit: 1000,
		})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		var nodes []GraphNode
		var links []GraphLink
		seenNodes := make(map[string]bool)

		for _, res := range results {
			n := res.Node
			if !seenNodes[n.GetId()] {
				seenNodes[n.GetId()] = true
				nodes = append(nodes, GraphNode{
					ID:        n.GetId(),
					Label:     n.GetLabel(),
					Summary:   n.GetSummary(),
					Type:      n.GetType(),
					Tense:     n.GetTemporal().GetTense().String(),
					Project:   n.GetDigitalLocation().GetProject(),
					AdapterID: n.GetAgentScope().GetCreatorAdapterId(),
				})
			}

			for _, edge := range res.ConnectedEdges {
				links = append(links, GraphLink{
					Source:   edge.GetSourceId(),
					Target:   edge.GetTargetId(),
					Relation: edge.GetRelation(),
					Weight:   edge.GetWeight(),
				})
			}
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(GraphData{Nodes: nodes, Links: links})
	})

	// HTML Dashboard Visualizer UI
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, htmlDashboard)
	})

	logPrintf("CriteriaDB Interactive Visualizer running at http://%s", addr)
	return http.ListenAndServe(addr, mux)
}

func logPrintf(format string, v ...any) {
	fmt.Printf("[Visualizer] "+format+"\n", v...)
}

const htmlDashboard = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <title>CriteriaDB — Agent Memory Graph Visualizer</title>
  <script src="https://cdn.jsdelivr.net/npm/d3@7"></script>
  <style>
    body { margin: 0; background: #0f172a; color: #f8fafc; font-family: system-ui, sans-serif; overflow: hidden; }
    #header { position: absolute; top: 16px; left: 16px; z-index: 10; background: rgba(15,23,42,0.85); backdrop-filter: blur(8px); padding: 12px 20px; border-radius: 12px; border: 1px solid #334155; }
    h1 { margin: 0 0 6px 0; font-size: 20px; color: #38bdf8; }
    p { margin: 0; font-size: 13px; color: #94a3b8; }
    #search { margin-top: 10px; width: 100%; padding: 8px 12px; background: #1e293b; border: 1px solid #475569; border-radius: 6px; color: #fff; box-sizing: border-box; }
    svg { width: 100vw; height: 100vh; }
    .node circle { stroke: #38bdf8; stroke-width: 2px; }
    .node text { fill: #f8fafc; font-size: 12px; pointer-events: none; }
    .link { stroke: #475569; stroke-opacity: 0.8; stroke-width: 1.5px; }
    #details { position: absolute; bottom: 16px; right: 16px; z-index: 10; background: rgba(15,23,42,0.9); padding: 16px; border-radius: 12px; border: 1px solid #334155; width: 320px; display: none; }
    #details h3 { margin: 0 0 8px 0; color: #38bdf8; font-size: 16px; }
    #details p { margin: 4px 0; font-size: 12px; color: #cbd5e1; }
  </style>
</head>
<body>
  <div id="header">
    <h1>CriteriaDB Memory Graph</h1>
    <p>Real-Time Agent Memory Visualizer</p>
    <input type="text" id="search" placeholder="Search memory nodes..." onkeyup="filterNodes(this.value)" />
  </div>
  <div id="details">
    <h3 id="det-title">Node Details</h3>
    <p id="det-id"></p>
    <p id="det-type"></p>
    <p id="det-tense"></p>
    <p id="det-project"></p>
    <p id="det-adapter"></p>
    <p id="det-summary"></p>
  </div>
  <svg></svg>
  <script>
    const width = window.innerWidth, height = window.innerHeight;
    const svg = d3.select("svg").attr("viewBox", [0, 0, width, height]);

    fetch("/api/graph")
      .then(res => res.json())
      .then(data => {
        const nodes = data.nodes || [];
        const links = data.links || [];

        const simulation = d3.forceSimulation(nodes)
          .force("link", d3.forceLink(links).id(function(d) { return d.id; }).distance(120))
          .force("charge", d3.forceManyBody().strength(-300))
          .force("center", d3.forceCenter(width / 2, height / 2));

        const link = svg.append("g")
          .selectAll("line")
          .data(links)
          .join("line")
          .attr("class", "link");

        const node = svg.append("g")
          .selectAll(".node")
          .data(nodes)
          .join("g")
          .attr("class", "node")
          .call(d3.drag()
            .on("start", dragstarted)
            .on("drag", dragged)
            .on("end", dragended));

        node.append("circle")
          .attr("r", 10)
          .attr("fill", function(d) { return d.tense === "TENSE_PAST" ? "#10b981" : d.tense === "TENSE_PLANNED" ? "#f59e0b" : "#3b82f6"; })
          .on("click", function(evt, d) { showDetails(d); });

        node.append("text")
          .attr("x", 14)
          .attr("y", 4)
          .text(function(d) { return d.label; });

        simulation.on("tick", function() {
          link
            .attr("x1", function(d) { return d.source.x; })
            .attr("y1", function(d) { return d.source.y; })
            .attr("x2", function(d) { return d.target.x; })
            .attr("y2", function(d) { return d.target.y; });

          node
            .attr("transform", function(d) { return "translate(" + d.x + "," + d.y + ")"; });
        });

        function dragstarted(event) {
          if (!event.active) simulation.alphaTarget(0.3).restart();
          event.subject.fx = event.subject.x;
          event.subject.fy = event.subject.y;
        }
        function dragged(event) {
          event.subject.fx = event.x;
          event.subject.fy = event.y;
        }
        function dragended(event) {
          if (!event.active) simulation.alphaTarget(0);
          event.subject.fx = null;
          event.subject.fy = null;
        }

        window.filterNodes = function(q) {
          q = q.toLowerCase();
          node.style("opacity", function(d) { return (d.label.toLowerCase().includes(q) || d.summary.toLowerCase().includes(q)) ? 1 : 0.15; });
        };

        function showDetails(d) {
          var det = document.getElementById("details");
          det.style.display = "block";
          document.getElementById("det-title").innerText = d.label;
          document.getElementById("det-id").innerText = "ID: " + d.id;
          document.getElementById("det-type").innerText = "Type: " + d.type;
          document.getElementById("det-tense").innerText = "Tense: " + d.tense;
          document.getElementById("det-project").innerText = "Project: " + (d.project || "default");
          document.getElementById("det-adapter").innerText = "Adapter: " + (d.adapter_id || "none");
          document.getElementById("det-summary").innerText = "Summary: " + d.summary;
        }
      });
  </script>
</body>
</html>`
