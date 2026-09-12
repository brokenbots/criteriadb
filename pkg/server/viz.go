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
    @import url('https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600;700&display=swap');
    body {
      margin: 0;
      background: #090d16;
      color: #f8fafc;
      font-family: 'Inter', system-ui, sans-serif;
      overflow: hidden;
    }
    #header {
      position: absolute;
      top: 16px;
      left: 16px;
      z-index: 10;
      background: rgba(15, 23, 42, 0.85);
      backdrop-filter: blur(12px);
      padding: 16px 20px;
      border-radius: 14px;
      border: 1px solid rgba(255, 255, 255, 0.1);
      box-shadow: 0 10px 30px rgba(0, 0, 0, 0.5);
      width: 340px;
    }
    h1 {
      margin: 0 0 4px 0;
      font-size: 18px;
      font-weight: 700;
      color: #38bdf8;
      display: flex;
      align-items: center;
      gap: 8px;
    }
    p.subtitle {
      margin: 0 0 12px 0;
      font-size: 12px;
      color: #94a3b8;
    }
    .filter-group {
      margin-bottom: 10px;
    }
    label {
      font-size: 11px;
      font-weight: 600;
      color: #cbd5e1;
      text-transform: uppercase;
      letter-spacing: 0.5px;
      display: block;
      margin-bottom: 4px;
    }
    input, select {
      width: 100%;
      padding: 8px 12px;
      background: #1e293b;
      border: 1px solid #334155;
      border-radius: 8px;
      color: #f8fafc;
      font-size: 13px;
      box-sizing: border-radius;
      outline: none;
      transition: all 0.2s ease;
    }
    input:focus, select:focus {
      border-color: #38bdf8;
      box-shadow: 0 0 0 2px rgba(56, 189, 248, 0.25);
    }
    .stats-pill {
      margin-top: 12px;
      font-size: 11px;
      color: #38bdf8;
      background: rgba(56, 189, 248, 0.1);
      padding: 6px 10px;
      border-radius: 6px;
      display: inline-block;
      font-weight: 600;
    }
    svg {
      width: 100vw;
      height: 100vh;
      cursor: grab;
    }
    svg:active {
      cursor: grabbing;
    }
    .node circle {
      stroke-width: 2px;
      transition: transform 0.2s ease, stroke-width 0.2s ease;
    }
    .node:hover circle {
      stroke-width: 4px;
      transform: scale(1.2);
    }
    .node text {
      fill: #f8fafc;
      font-size: 12px;
      font-weight: 500;
      pointer-events: none;
      text-shadow: 0 2px 4px rgba(0,0,0,0.8);
    }
    .link {
      stroke: #334155;
      stroke-opacity: 0.8;
      stroke-width: 1.5px;
    }
    .link-label {
      fill: #64748b;
      font-size: 10px;
      font-weight: 600;
      pointer-events: none;
    }
    #details {
      position: absolute;
      bottom: 16px;
      right: 16px;
      z-index: 10;
      background: rgba(15, 23, 42, 0.9);
      backdrop-filter: blur(12px);
      padding: 20px;
      border-radius: 14px;
      border: 1px solid rgba(255, 255, 255, 0.1);
      box-shadow: 0 10px 30px rgba(0, 0, 0, 0.6);
      width: 360px;
      max-height: 480px;
      overflow-y: auto;
      display: none;
    }
    #details .close-btn {
      float: right;
      cursor: pointer;
      font-size: 16px;
      color: #94a3b8;
    }
    #details h3 {
      margin: 0 0 12px 0;
      color: #38bdf8;
      font-size: 16px;
      font-weight: 700;
    }
    .badge {
      display: inline-block;
      padding: 3px 8px;
      border-radius: 4px;
      font-size: 10px;
      font-weight: 700;
      text-transform: uppercase;
      margin-right: 6px;
    }
    .badge-tense { background: #1e293b; color: #38bdf8; border: 1px solid #38bdf8; }
    .badge-type { background: #1e293b; color: #10b981; border: 1px solid #10b981; }
    .det-row {
      margin: 8px 0;
      font-size: 12px;
      color: #cbd5e1;
      line-height: 1.5;
    }
    .det-row span {
      color: #94a3b8;
      font-weight: 600;
    }
  </style>
</head>
<body>
  <div id="header">
    <h1>CriteriaDB Visualizer</h1>
    <p class="subtitle">Agent Memory Graph Dashboard</p>
    
    <div class="filter-group">
      <label>Search Memory Nodes</label>
      <input type="text" id="search" placeholder="Type label or summary..." oninput="applyFilters()" />
    </div>

    <div class="filter-group">
      <label>Filter by Node Type</label>
      <select id="filter-type" onchange="applyFilters()">
        <option value="all">All Types</option>
        <option value="fact">fact</option>
        <option value="event">event</option>
        <option value="devops_task">devops_task</option>
        <option value="verification_passed">verification_passed</option>
        <option value="consolidated_fact">consolidated_fact</option>
      </select>
    </div>

    <div class="filter-group">
      <label>Filter by Temporal Tense</label>
      <select id="filter-tense" onchange="applyFilters()">
        <option value="all">All Tenses</option>
        <option value="TENSE_PAST">TENSE_PAST</option>
        <option value="TENSE_PRESENT">TENSE_PRESENT</option>
        <option value="TENSE_PLANNED">TENSE_PLANNED</option>
        <option value="TENSE_FUTURE">TENSE_FUTURE</option>
      </select>
    </div>

    <div id="stats" class="stats-pill">Loading graph...</div>
  </div>

  <div id="details">
    <span class="close-btn" onclick="hideDetails()">&times;</span>
    <h3 id="det-title">Node Details</h3>
    <div>
      <span id="det-tense-badge" class="badge badge-tense">TENSE_PRESENT</span>
      <span id="det-type-badge" class="badge badge-type">FACT</span>
    </div>
    <div class="det-row"><span>Node ID:</span> <div id="det-id"></div></div>
    <div class="det-row"><span>Project:</span> <div id="det-project"></div></div>
    <div class="det-row"><span>Adapter:</span> <div id="det-adapter"></div></div>
    <div class="det-row"><span>Summary:</span> <div id="det-summary"></div></div>
    <div class="det-row"><span>Connected Relations:</span> <div id="det-edges"></div></div>
  </div>

  <svg></svg>

  <script>
    const width = window.innerWidth, height = window.innerHeight;
    const svg = d3.select("svg");
    const container = svg.append("g");

    // Enable Zoom and Pan
    svg.call(d3.zoom()
      .scaleExtent([0.1, 4])
      .on("zoom", (event) => {
        container.attr("transform", event.transform);
      }));

    let allNodes = [], allLinks = [], graphNodeSel, graphLinkSel;

    fetch("/api/graph")
      .then(res => res.json())
      .then(data => {
        allNodes = data.nodes || [];
        allLinks = data.links || [];

        const simulation = d3.forceSimulation(allNodes)
          .force("link", d3.forceLink(allLinks).id(d => d.id).distance(140))
          .force("charge", d3.forceManyBody().strength(-350))
          .force("center", d3.forceCenter(width / 2, height / 2));

        graphLinkSel = container.append("g")
          .selectAll("line")
          .data(allLinks)
          .join("line")
          .attr("class", "link");

        graphNodeSel = container.append("g")
          .selectAll(".node")
          .data(allNodes)
          .join("g")
          .attr("class", "node")
          .call(d3.drag()
            .on("start", dragstarted)
            .on("drag", dragged)
            .on("end", dragended));

        graphNodeSel.append("circle")
          .attr("r", 12)
          .attr("fill", d => getNodeColor(d))
          .attr("stroke", d => d3.rgb(getNodeColor(d)).brighter(1))
          .on("click", (evt, d) => showDetails(d));

        graphNodeSel.append("text")
          .attr("x", 16)
          .attr("y", 4)
          .text(d => d.label);

        simulation.on("tick", () => {
          graphLinkSel
            .attr("x1", d => d.source.x)
            .attr("y1", d => d.source.y)
            .attr("x2", d => d.target.x)
            .attr("y2", d => d.target.y);

          graphNodeSel
            .attr("transform", d => "translate(" + d.x + "," + d.y + ")");
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

        updateStats(allNodes.length, allLinks.length);
      });

    function getNodeColor(d) {
      if (d.type === "consolidated_fact") return "#a855f7";
      if (d.tense === "TENSE_PAST") return "#10b981";
      if (d.tense === "TENSE_PLANNED") return "#f59e0b";
      return "#3b82f6";
    }

    window.applyFilters = function() {
      const q = document.getElementById("search").value.toLowerCase();
      const typeVal = document.getElementById("filter-type").value;
      const tenseVal = document.getElementById("filter-tense").value;

      let visibleCount = 0;
      graphNodeSel.style("opacity", d => {
        const matchesQuery = !q || d.label.toLowerCase().includes(q) || d.summary.toLowerCase().includes(q);
        const matchesType = typeVal === "all" || d.type === typeVal;
        const matchesTense = tenseVal === "all" || d.tense === tenseVal;

        const visible = matchesQuery && matchesType && matchesTense;
        if (visible) visibleCount++;
        return visible ? 1 : 0.1;
      });

      updateStats(visibleCount, allLinks.length);
    };

    function updateStats(nodesCount, linksCount) {
      document.getElementById("stats").innerText = "Displaying " + nodesCount + " Nodes • " + linksCount + " Edges";
    }

    function showDetails(d) {
      const det = document.getElementById("details");
      det.style.display = "block";
      document.getElementById("det-title").innerText = d.label;
      document.getElementById("det-id").innerText = d.id;
      document.getElementById("det-project").innerText = d.project || "default";
      document.getElementById("det-adapter").innerText = d.adapter_id || "none";
      document.getElementById("det-summary").innerText = d.summary;
      document.getElementById("det-tense-badge").innerText = d.tense;
      document.getElementById("det-type-badge").innerText = d.type;

      // Find connected relations
      const rels = allLinks.filter(l => (l.source.id === d.id || l.source === d.id || l.target.id === d.id || l.target === d.id));
      let relHTML = "";
      if (rels.length === 0) {
        relHTML = "<em>No edges connected</em>";
      } else {
        rels.forEach(l => {
        const src = l.source.id || l.source;
        const tgt = l.target.id || l.target;
        relHTML += "<div>&bull; <strong>" + l.relation + "</strong> (" + src + " &rarr; " + tgt + ")</div>";
      });
      }
      document.getElementById("det-edges").innerHTML = relHTML;
    }

    window.hideDetails = function() {
      document.getElementById("details").style.display = "none";
    };
  </script>
</body>
</html>`
