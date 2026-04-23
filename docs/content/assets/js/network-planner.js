(function () {
  "use strict";

  function init() {
    var root = document.getElementById("network-planner");
    if (!root) return;

    var NS = "http://www.w3.org/2000/svg";
    var state = {};
    var BOX_H = 48;
    var GAP = 14;

    var FILL = {
      cp:    { bg: "#dbeafe", border: "#2563eb" },
      dp:    { bg: "#dcfce7", border: "#16a34a" },
      infra: { bg: "#fef3c7", border: "#d97706" },
      bmc:   { bg: "#fce4ec", border: "#c62828" },
      def:   { bg: "#f5f5f5", border: "#9e9e9e" },
      actor: { bg: "#fff3e0", border: "#e65100" },
    };

    var NC = {
      ext: "#64b5f6", mgmt: "#ba68c8", dp: "#81c784",
      bmc: "#ef9a9a", plink: "#ffb74d", multus: "#90a4ae",
      storage: "#4db6ac",
    };

    // ── Steps ────────────────────────────────────

    var STEPS = {
      platform: {
        id: "platform", title: "Platform",
        options: [
          { value: "aws", label: "AWS" },
          { value: "azure", label: "Azure" },
          { value: "agent", label: "Agent / Bare Metal" },
          { value: "kubevirt", label: "KubeVirt" },
        ],
      },
      awsTopology: {
        id: "topology", title: "Endpoint Access",
        options: [
          { value: "public", label: "Public" },
          { value: "publicandprivate", label: "Public and Private" },
          { value: "private", label: "Private" },
        ],
      },
      azureTopology: {
        id: "topology", title: "Endpoint Access",
        options: [
          { value: "public", label: "Public" },
          { value: "private", label: "Private" },
        ],
      },
      agentPublishing: {
        id: "publishing", title: "Service Publishing",
        options: [
          { value: "nodeport", label: "NodePort (default)" },
          { value: "loadbalancer", label: "LoadBalancer (production)" },
        ],
      },
      kubevirtMode: {
        id: "kvmode", title: "Networking Mode",
        options: [
          { value: "passthrough", label: "Default (BaseDomainPassthrough)" },
          { value: "custom", label: "Custom Domain + LB" },
          { value: "isolated", label: "Isolated (Multus)" },
        ],
      },
      awsExtDns: {
        id: "extdns", title: "External DNS",
        options: [
          { value: "no", label: "No (default)" },
          { value: "yes", label: "Yes (--external-dns-domain)" },
        ],
      },
    };

    function getActiveSteps() {
      var s = [STEPS.platform];
      if (!state.platform) return s;
      if (state.platform === "aws") {
        s.push(STEPS.awsTopology);
        if (state.topology === "public") s.push(STEPS.awsExtDns);
      } else if (state.platform === "azure") {
        s.push(STEPS.azureTopology);
      } else if (state.platform === "agent") {
        s.push(STEPS.agentPublishing);
      } else if (state.platform === "kubevirt") {
        s.push(STEPS.kubevirtMode);
      }
      return s;
    }

    function isComplete() {
      if (!state.platform) return false;
      if (state.platform === "aws") {
        if (!state.topology) return false;
        if (state.topology === "public") return !!state.extdns;
        return true;
      }
      if (state.platform === "azure") return !!state.topology;
      if (state.platform === "agent") return !!state.publishing;
      if (state.platform === "kubevirt") return !!state.kvmode;
      return false;
    }

    function variantKey() {
      if (state.platform === "aws") {
        if (state.topology === "public") return "aws_public_" + state.extdns;
        if (state.topology === "publicandprivate") return "aws_publicandprivate";
        return "aws_private";
      }
      if (state.platform === "azure") return "azure_" + state.topology;
      if (state.platform === "agent") return "agent_" + state.publishing;
      if (state.platform === "kubevirt") return "kv_" + state.kvmode;
      return null;
    }

    // ── Rendering ────────────────────────────────

    function render() {
      root.innerHTML = "";

      var stepsEl = document.createElement("div");
      stepsEl.className = "np-steps";
      var active = getActiveSteps();
      var lastStepEl = null;
      for (var i = 0; i < active.length; i++) {
        lastStepEl = buildStepEl(active[i], i, active);
        stepsEl.appendChild(lastStepEl);
        if (!state[active[i].id]) break;
      }
      root.appendChild(stepsEl);

      if (isComplete()) {
        var diagramEl = document.createElement("div");
        diagramEl.className = "np-diagram";
        var key = variantKey();
        if (DIAGRAMS[key]) renderSVG(diagramEl, DIAGRAMS[key]());
        root.appendChild(diagramEl);
        root.appendChild(buildSummaryTable());
      } else {
        var hint = document.createElement("div");
        hint.className = "np-diagram";
        hint.innerHTML = '<p class="np-hint">Select options above to build your network diagram</p>';
        root.appendChild(hint);
      }

      if (lastStepEl) {
        var target = lastStepEl.querySelector(".np-opt--active") ||
                     lastStepEl.querySelector(".np-opt");
        if (target) target.focus();
      }
    }

    function buildStepEl(step, index, allSteps) {
      var el = document.createElement("div");
      el.className = "np-step";
      var hdr = document.createElement("div");
      hdr.className = "np-step-header";
      hdr.innerHTML = '<span class="np-step-num">' + (index + 1) + "</span>" + step.title;
      el.appendChild(hdr);
      var opts = document.createElement("div");
      opts.className = "np-options";
      step.options.forEach(function (o) {
        var btn = document.createElement("button");
        btn.type = "button";
        btn.setAttribute("aria-pressed", state[step.id] === o.value ? "true" : "false");
        btn.className = "np-opt" + (state[step.id] === o.value ? " np-opt--active" : "");
        btn.textContent = o.label;
        btn.addEventListener("click", function () {
          state[step.id] = o.value;
          for (var j = index + 1; j < allSteps.length; j++) delete state[allSteps[j].id];
          render();
        });
        opts.appendChild(btn);
      });
      el.appendChild(opts);
      return el;
    }

    function buildSummaryTable() {
      var key = variantKey();
      var rows = SUMMARIES[key] || [];
      var wrap = document.createElement("div");
      wrap.className = "np-summary";
      var t = document.createElement("div");
      t.className = "np-summary-title";
      t.textContent = "Service Publishing Summary";
      wrap.appendChild(t);
      var tbl = document.createElement("table");
      tbl.className = "np-table";
      var thead = document.createElement("thead");
      var hr = document.createElement("tr");
      ["Service", "Strategy", "Port", "Details"].forEach(function (h) {
        var th = document.createElement("th");
        th.textContent = h;
        hr.appendChild(th);
      });
      thead.appendChild(hr);
      tbl.appendChild(thead);
      var tb = document.createElement("tbody");
      rows.forEach(function (r) {
        var tr = document.createElement("tr");
        var tdSvc = document.createElement("td");
        tdSvc.textContent = r.svc;
        var tdStrat = document.createElement("td");
        var code = document.createElement("code");
        code.textContent = r.strategy;
        tdStrat.appendChild(code);
        var tdPort = document.createElement("td");
        tdPort.textContent = r.port;
        var tdDetail = document.createElement("td");
        tdDetail.textContent = r.detail;
        tr.appendChild(tdSvc);
        tr.appendChild(tdStrat);
        tr.appendChild(tdPort);
        tr.appendChild(tdDetail);
        tb.appendChild(tr);
      });
      tbl.appendChild(tb);
      wrap.appendChild(tbl);
      return wrap;
    }

    // ── SVG Engine ───────────────────────────────

    function svgEl(tag, attrs, text) {
      var e = document.createElementNS(NS, tag);
      if (attrs) for (var k in attrs) e.setAttribute(k, attrs[k]);
      if (text !== undefined) e.textContent = text;
      return e;
    }

    function findNet(nets, id) {
      for (var i = 0; i < nets.length; i++) if (nets[i].id === id) return nets[i];
      return null;
    }

    function renderSVG(container, cfg) {
      var W = cfg.width || 880;
      var H = cfg.height || 460;

      var svg = svgEl("svg", {
        viewBox: "0 0 " + W + " " + H, xmlns: NS,
        role: "img",
        "aria-label": cfg.title + (cfg.subtitle ? ". " + cfg.subtitle : ""),
      });
      svg.style.cssText = "width:100%;max-width:" + W + "px;font-family:-apple-system,system-ui,sans-serif";

      // Defs: arrowhead
      var defs = svgEl("defs");
      var mkr = svgEl("marker", {
        id: "np-arr", viewBox: "0 0 10 10", refX: "9", refY: "5",
        markerWidth: "5", markerHeight: "5", orient: "auto",
      });
      mkr.appendChild(svgEl("path", { d: "M0,1 L10,5 L0,9z", fill: "#999" }));
      defs.appendChild(mkr);
      svg.appendChild(defs);

      // Background
      svg.appendChild(svgEl("rect", { width: W, height: H, fill: "#fafafa", rx: "8" }));

      // Title
      if (cfg.title) {
        svg.appendChild(svgEl("text", {
          x: W / 2, y: "22", "text-anchor": "middle",
          "font-size": "14", "font-weight": "700", fill: "#333",
        }, cfg.title));
      }
      if (cfg.subtitle) {
        svg.appendChild(svgEl("text", {
          x: W / 2, y: "38", "text-anchor": "middle",
          "font-size": "10", "font-weight": "500", fill: "#666",
          "font-style": "italic",
        }, cfg.subtitle));
      }

      var pos = {};

      // ── Draw network buses ──
      (cfg.networks || []).forEach(function (n) {
        svg.appendChild(svgEl("line", {
          x1: "35", y1: n.y, x2: W - 35, y2: n.y,
          stroke: n.color, "stroke-width": n.dashed ? "2" : "3",
          "stroke-dasharray": n.dashed ? "8,4" : "none", opacity: "0.75",
        }));
        svg.appendChild(svgEl("text", {
          x: "40", y: n.y - 6,
          "font-size": "10", "font-weight": "600", fill: n.color, opacity: "0.9",
        }, n.label));
      });

      // ── Draw component rows ──
      (cfg.rows || []).forEach(function (row) {
        var items = row.items;
        var tw = 0;
        items.forEach(function (it) { tw += (it.w || 100); });
        tw += (items.length - 1) * GAP;
        var cx = (W - tw) / 2;

        items.forEach(function (it) {
          var w = it.w || 100;
          var h = it.h || BOX_H;
          var x = cx, y = row.y;
          var st = FILL[it.style] || FILL.def;

          // Box
          svg.appendChild(svgEl("rect", {
            x: x, y: y, width: w, height: h, rx: "5",
            fill: st.bg, stroke: st.border, "stroke-width": "1.5",
          }));

          // Label
          var ty = it.sub ? y + h / 2 - 3 : y + h / 2 + 4;
          svg.appendChild(svgEl("text", {
            x: x + w / 2, y: ty, "text-anchor": "middle",
            "font-size": "11", "font-weight": "600", fill: "#333",
          }, it.label));
          if (it.sub) {
            svg.appendChild(svgEl("text", {
              x: x + w / 2, y: ty + 14, "text-anchor": "middle",
              "font-size": "9", fill: "#777",
            }, it.sub));
          }

          // User-access badge
          if (it.user) {
            var bx = x + w - 14, by = y + 3;
            svg.appendChild(svgEl("circle", {
              cx: bx + 4, cy: by + 3, r: "2.5",
              fill: "#e65100", opacity: "0.85",
            }));
            svg.appendChild(svgEl("path", {
              d: "M" + (bx) + "," + (by + 11) + " Q" + (bx + 4) + "," + (by + 6) + " " + (bx + 8) + "," + (by + 11),
              stroke: "#e65100", "stroke-width": "1.5", fill: "none", opacity: "0.85",
            }));
          }

          // Leg to network bus
          var net = findNet(cfg.networks, row.network);
          if (net) {
            var ly1 = net.y < y ? y : y + h;
            svg.appendChild(svgEl("line", {
              x1: x + w / 2, y1: ly1, x2: x + w / 2, y2: net.y,
              stroke: net.color, "stroke-width": "1.5", opacity: "0.5",
            }));
          }

          pos[it.id] = { cx: x + w / 2, top: y, bot: y + h };
          cx += w + GAP;
        });
      });

      // ── Draw floating components ──
      (cfg.floats || []).forEach(function (f) {
        var st = FILL[f.style] || FILL.def;
        var fh = f.h || 35;
        svg.appendChild(svgEl("rect", {
          x: f.x, y: f.y, width: f.w, height: fh, rx: "5",
          fill: st.bg, stroke: st.border, "stroke-width": "1.5",
        }));
        svg.appendChild(svgEl("text", {
          x: f.x + f.w / 2, y: f.y + fh / 2 + 4, "text-anchor": "middle",
          "font-size": "10", "font-weight": "600", fill: "#333",
        }, f.label));
        pos[f.id] = { cx: f.x + f.w / 2, top: f.y, bot: f.y + fh };
      });

      // ── Draw extra legs (secondary network connections) ──
      (cfg.extraLegs || []).forEach(function (xl) {
        var c = pos[xl.comp];
        var n = findNet(cfg.networks, xl.net);
        if (!c || !n) return;
        var y1 = n.y > c.bot ? c.bot : c.top;
        var legStyle = {
          stroke: n.color, "stroke-width": "1.5", opacity: "0.5", "stroke-dasharray": "4,3",
        };
        function legLine(x1, y1, x2, y2) {
          var a = {}; for (var k in legStyle) a[k] = legStyle[k];
          a.x1 = x1; a.y1 = y1; a.x2 = x2; a.y2 = y2;
          svg.appendChild(svgEl("line", a));
        }
        if (xl.dx) {
          var xEnd = c.cx + xl.dx;
          if (xl.startDy) {
            var yRoute = y1 + xl.startDy;
            legLine(c.cx, y1, c.cx, yRoute);
            legLine(c.cx, yRoute, xEnd, yRoute);
            legLine(xEnd, yRoute, xEnd, n.y);
          } else {
            legLine(c.cx, y1, xEnd, y1);
            legLine(xEnd, y1, xEnd, n.y);
          }
        } else {
          legLine(c.cx, y1, c.cx, n.y);
        }
      });

      // ── Draw links (cross-network connections) ──
      (cfg.links || []).forEach(function (lk) {
        var a = pos[lk.from], b = pos[lk.to];
        if (!a || !b) return;

        var y1, y2;
        if (a.bot <= b.top) { y1 = a.bot; y2 = b.top; }
        else if (b.bot <= a.top) { y1 = a.top; y2 = b.bot; }
        else { y1 = (a.top + a.bot) / 2; y2 = (b.top + b.bot) / 2; }

        svg.appendChild(svgEl("line", {
          x1: a.cx, y1: y1, x2: b.cx, y2: y2,
          stroke: lk.color || "#aaa", "stroke-width": lk.bold ? "2.5" : "1.5",
          "stroke-dasharray": lk.dashed ? "6,3" : "none",
          "marker-end": "url(#np-arr)",
        }));

        if (lk.label) {
          var t = lk.labelPos != null ? lk.labelPos : 0.3;
          var mx = a.cx + (b.cx - a.cx) * t + (lk.labelDx || 0);
          var my = y1 + (y2 - y1) * t + (lk.labelDy || 0);
          var off = Math.abs(a.cx - b.cx) < 30 ? 12 : 5;
          svg.appendChild(svgEl("text", {
            x: mx + off, y: my, "font-size": "9", fill: "#999",
          }, lk.label));
        }
      });

      // ── Legend ──
      var legend = [
        { label: "Contact Point", c: FILL.cp },
        { label: "Data Plane", c: FILL.dp },
        { label: "Infrastructure", c: FILL.infra },
        { label: "Traffic Actor", c: FILL.actor },
      ];
      var legendItems = legend.length + 1;
      var lx = W / 2 - (legendItems * 120) / 2;
      var ly = H - 18;
      legend.forEach(function (it) {
        svg.appendChild(svgEl("rect", {
          x: lx, y: ly - 8, width: "12", height: "12", rx: "2",
          fill: it.c.bg, stroke: it.c.border, "stroke-width": "1",
        }));
        svg.appendChild(svgEl("text", {
          x: lx + 16, y: ly + 2, "font-size": "10", fill: "#888",
        }, it.label));
        lx += 120;
      });
      // User access legend entry
      svg.appendChild(svgEl("circle", {
        cx: lx + 4, cy: ly - 5, r: "2.5",
        fill: "#e65100", opacity: "0.85",
      }));
      svg.appendChild(svgEl("path", {
        d: "M" + lx + "," + (ly + 3) + " Q" + (lx + 4) + "," + (ly - 2) + " " + (lx + 8) + "," + (ly + 3),
        stroke: "#e65100", "stroke-width": "1.5", fill: "none", opacity: "0.85",
      }));
      svg.appendChild(svgEl("text", {
        x: lx + 14, y: ly + 2, "font-size": "10", fill: "#888",
      }, "User Access"));

      container.innerHTML = "";
      container.appendChild(svg);
    }

    // ── Diagram configs ──────────────────────────

    var DIAGRAMS = {};

    DIAGRAMS.aws_public_no = function () {
      return {
        title: "AWS — Public (no External DNS)", height: 540,
        subtitle: "Publishing: API → LoadBalancer  |  OAuth, Konnectivity, Ignition → Route",
        networks: [
          { id: "ext", y: 100, label: "Public / External Network", color: NC.ext },
          { id: "mgmt", y: 250, label: "Management Cluster Network", color: NC.mgmt },
          { id: "dp", y: 410, label: "Worker VPC", color: NC.dp },
        ],
        rows: [
          { network: "ext", y: 120, items: [
            { id: "elb", label: "External LB", w: 110, style: "infra" },
            { id: "mgi", label: "Mgmt Ingress", sub: "OCP Router", w: 120, style: "infra" },
          ]},
          { network: "mgmt", y: 270, items: [
            { id: "kas", label: "API", sub: ":6443 · LB", w: 100, style: "cp", user: true },
            { id: "oauth", label: "OAuth", sub: ":443 · Route", w: 100, style: "cp", user: true },
            { id: "kc", label: "Konnectivity", sub: ":443 · Route", w: 120, style: "cp" },
            { id: "ign", label: "Ignition", sub: ":443 · Route", w: 100, style: "cp" },
          ]},
          { network: "dp", y: 430, items: [
            { id: "w1", label: "Worker 1", w: 100, h: 58, style: "dp" },
            { id: "w2", label: "Worker 2", w: 100, h: 58, style: "dp" },
            { id: "dpi", label: "DP Ingress", sub: ":80/:443", w: 110, style: "dp", user: true },
          ]},
        ],
        floats: [
          { id: "users", label: "External Users", x: 370, y: 46, w: 140, h: 26, style: "actor" },
        ],
        links: [
          { from: "users", to: "elb", label: "API clients" },
          { from: "users", to: "mgi", label: "app users" },
          { from: "elb", to: "kas" },
          { from: "mgi", to: "oauth" },
          { from: "mgi", to: "kc" },
          { from: "mgi", to: "ign" },
          { from: "w1", to: "kc", label: "reverse tunnel", dashed: true },
          { from: "w2", to: "kc", dashed: true },
        ],
      };
    };

    DIAGRAMS.aws_public_yes = function () {
      return {
        title: "AWS — Public (External DNS)", height: 540,
        subtitle: "Publishing: All services → Route (via HCP Router, External DNS creates Route53 hostnames)",
        networks: [
          { id: "ext", y: 100, label: "Public / External Network", color: NC.ext },
          { id: "mgmt", y: 250, label: "Management Cluster Network", color: NC.mgmt },
          { id: "dp", y: 410, label: "Worker VPC", color: NC.dp },
        ],
        rows: [
          { network: "ext", y: 120, items: [
            { id: "elb", label: "External LB", w: 120, style: "infra" },
            { id: "hcpr", label: "HCP Router", w: 120, style: "infra" },
          ]},
          { network: "mgmt", y: 270, items: [
            { id: "kas", label: "API", sub: ":6443 · Route", w: 100, style: "cp", user: true },
            { id: "oauth", label: "OAuth", sub: ":443 · Route", w: 100, style: "cp", user: true },
            { id: "kc", label: "Konnectivity", sub: ":443 · Route", w: 120, style: "cp" },
            { id: "ign", label: "Ignition", sub: ":443 · Route", w: 100, style: "cp" },
          ]},
          { network: "dp", y: 430, items: [
            { id: "w1", label: "Worker 1", w: 100, h: 58, style: "dp" },
            { id: "w2", label: "Worker 2", w: 100, h: 58, style: "dp" },
            { id: "dpi", label: "DP Ingress", sub: ":80/:443", w: 110, style: "dp", user: true },
          ]},
        ],
        floats: [
          { id: "users", label: "External Users", x: 370, y: 46, w: 140, h: 26, style: "actor" },
        ],
        links: [
          { from: "users", to: "elb", label: "all traffic" },
          { from: "elb", to: "hcpr" },
          { from: "hcpr", to: "kas" },
          { from: "hcpr", to: "oauth" },
          { from: "hcpr", to: "kc" },
          { from: "hcpr", to: "ign" },
          { from: "w1", to: "kc", label: "reverse tunnel", dashed: true },
          { from: "w2", to: "kc", dashed: true },
        ],
      };
    };

    DIAGRAMS.aws_publicandprivate = function () {
      return {
        title: "AWS — Public and Private Topology", height: 625,
        subtitle: "Publishing: API → LoadBalancer (public)  |  DP access via PrivateLink",
        networks: [
          { id: "ext", y: 100, label: "Public / External Network", color: NC.ext },
          { id: "mgmt", y: 235, label: "Management Cluster Network", color: NC.mgmt },
          { id: "plink", y: 385, label: "AWS PrivateLink", color: NC.plink },
          { id: "dp", y: 495, label: "Worker VPC", color: NC.dp },
        ],
        rows: [
          { network: "ext", y: 120, items: [
            { id: "elb", label: "External LB", w: 120, style: "infra" },
            { id: "mgi", label: "Mgmt Ingress", sub: "OCP Router", w: 120, style: "infra" },
          ]},
          { network: "mgmt", y: 255, items: [
            { id: "kas", label: "API", sub: ":6443 · LB", w: 90, style: "cp", user: true },
            { id: "oauth", label: "OAuth", sub: ":443 · Route", w: 90, style: "cp", user: true },
            { id: "kc", label: "Konnectivity", sub: ":443 · Route", w: 110, style: "cp" },
            { id: "ign", label: "Ignition", sub: ":443 · Route", w: 90, style: "cp" },
            { id: "hcpr", label: "HCP Router", w: 100, style: "infra" },
          ]},
          { network: "plink", y: 405, items: [
            { id: "vep", label: "VPC Endpoint", w: 120, style: "infra" },
          ]},
          { network: "dp", y: 515, items: [
            { id: "w1", label: "Worker 1", w: 100, h: 58, style: "dp" },
            { id: "w2", label: "Worker 2", w: 100, h: 58, style: "dp" },
            { id: "dpi", label: "DP Ingress", sub: ":80/:443", w: 110, style: "dp", user: true },
          ]},
        ],
        floats: [
          { id: "users", label: "External Users", x: 370, y: 46, w: 140, h: 26, style: "actor" },
          { id: "ilb", label: "Internal LB", x: 580, y: 310, w: 100, h: 35, style: "infra" },
        ],
        links: [
          { from: "users", to: "elb", label: "API clients" },
          { from: "users", to: "mgi", label: "app users" },
          { from: "elb", to: "kas", label: "public" },
          { from: "mgi", to: "oauth" },
          { from: "mgi", to: "kc" },
          { from: "mgi", to: "ign" },
          { from: "vep", to: "ilb", label: "PrivateLink" },
          { from: "ilb", to: "hcpr" },
          { from: "w1", to: "vep", dashed: true },
          { from: "w2", to: "vep", dashed: true },
          { from: "w1", to: "kc", label: "reverse tunnel", dashed: true },
          { from: "w2", to: "kc", dashed: true },
        ],
      };
    };

    DIAGRAMS.aws_private = function () {
      return {
        title: "AWS — Private Topology", height: 560,
        subtitle: "Publishing: All services → Route (via PrivateLink, no public access — requires VPN/DirectConnect)",
        networks: [
          { id: "mgmt", y: 100, label: "Management Cluster Network", color: NC.mgmt },
          { id: "plink", y: 280, label: "AWS PrivateLink", color: NC.plink },
          { id: "dp", y: 430, label: "Worker VPC", color: NC.dp },
        ],
        rows: [
          { network: "mgmt", y: 120, items: [
            { id: "kas", label: "API", sub: ":6443 · Route", w: 90, style: "cp", user: true },
            { id: "oauth", label: "OAuth", sub: ":443 · Route", w: 90, style: "cp", user: true },
            { id: "kc", label: "Konnectivity", sub: ":443 · Route", w: 110, style: "cp" },
            { id: "ign", label: "Ignition", sub: ":443 · Route", w: 90, style: "cp" },
            { id: "hcpr", label: "HCP Router", w: 100, style: "infra" },
          ]},
          { network: "plink", y: 300, items: [
            { id: "vep", label: "VPC Endpoint", w: 120, style: "infra" },
          ]},
          { network: "dp", y: 450, items: [
            { id: "w1", label: "Worker 1", w: 100, h: 58, style: "dp" },
            { id: "w2", label: "Worker 2", w: 100, h: 58, style: "dp" },
            { id: "dpi", label: "DP Ingress", sub: ":80/:443", w: 110, style: "dp", user: true },
          ]},
        ],
        floats: [
          { id: "users", label: "VPN / DC Users", x: 355, y: 46, w: 170, h: 26, style: "actor" },
          { id: "ilb", label: "Internal LB", x: 560, y: 205, w: 100, h: 35, style: "infra" },
        ],
        links: [
          { from: "users", to: "vep", label: "via VPN", labelPos: 0.7 },
          { from: "vep", to: "ilb", label: "PrivateLink" },
          { from: "ilb", to: "hcpr" },
          { from: "w1", to: "vep", dashed: true },
          { from: "w2", to: "vep", dashed: true },
          { from: "w1", to: "kc", label: "reverse tunnel", dashed: true },
          { from: "w2", to: "kc", dashed: true },
        ],
      };
    };

    DIAGRAMS.azure_public = function () {
      return {
        title: "Azure — Public Topology", height: 540,
        subtitle: "Publishing: All services → Route",
        networks: [
          { id: "ext", y: 100, label: "Public / External Network", color: NC.ext },
          { id: "mgmt", y: 250, label: "Management Cluster Network", color: NC.mgmt },
          { id: "dp", y: 410, label: "Worker VNET", color: NC.dp },
        ],
        rows: [
          { network: "ext", y: 120, items: [
            { id: "mgi", label: "Mgmt Ingress", sub: "OCP Router", w: 130, style: "infra" },
          ]},
          { network: "mgmt", y: 270, items: [
            { id: "kas", label: "API", sub: ":6443 · Route", w: 100, style: "cp", user: true },
            { id: "oauth", label: "OAuth", sub: ":443 · Route", w: 100, style: "cp", user: true },
            { id: "kc", label: "Konnectivity", sub: ":443 · Route", w: 120, style: "cp" },
            { id: "ign", label: "Ignition", sub: ":443 · Route", w: 100, style: "cp" },
          ]},
          { network: "dp", y: 430, items: [
            { id: "w1", label: "Worker 1", w: 100, h: 58, style: "dp" },
            { id: "w2", label: "Worker 2", w: 100, h: 58, style: "dp" },
            { id: "dpi", label: "DP Ingress", sub: ":80/:443", w: 110, style: "dp", user: true },
          ]},
        ],
        floats: [
          { id: "users", label: "External Users", x: 370, y: 46, w: 140, h: 26, style: "actor" },
        ],
        links: [
          { from: "users", to: "mgi", label: "all traffic" },
          { from: "mgi", to: "kas" },
          { from: "mgi", to: "oauth" },
          { from: "mgi", to: "kc" },
          { from: "mgi", to: "ign" },
          { from: "w1", to: "kc", label: "reverse tunnel", dashed: true },
          { from: "w2", to: "kc", dashed: true },
        ],
      };
    };

    DIAGRAMS.azure_private = function () {
      return {
        title: "Azure — Private Topology", height: 560,
        subtitle: "Publishing: All services → Route (via Private Link)",
        networks: [
          { id: "mgmt", y: 100, label: "Management Cluster Network", color: NC.mgmt },
          { id: "plink", y: 280, label: "Azure Private Link", color: NC.plink },
          { id: "dp", y: 430, label: "Worker VNET", color: NC.dp },
        ],
        rows: [
          { network: "mgmt", y: 120, items: [
            { id: "kas", label: "API", sub: ":6443 · Route", w: 90, style: "cp", user: true },
            { id: "oauth", label: "OAuth", sub: ":443 · Route", w: 90, style: "cp", user: true },
            { id: "kc", label: "Konnectivity", sub: ":443 · Route", w: 110, style: "cp" },
            { id: "ign", label: "Ignition", sub: ":443 · Route", w: 90, style: "cp" },
            { id: "hcpr", label: "HCP Router", w: 100, style: "infra" },
          ]},
          { network: "plink", y: 300, items: [
            { id: "pe", label: "Private Endpoint", w: 130, style: "infra" },
          ]},
          { network: "dp", y: 450, items: [
            { id: "w1", label: "Worker 1", w: 100, h: 58, style: "dp" },
            { id: "w2", label: "Worker 2", w: 100, h: 58, style: "dp" },
            { id: "dpi", label: "DP Ingress", sub: ":80/:443", w: 110, style: "dp", user: true },
          ]},
        ],
        floats: [
          { id: "users", label: "VPN / DC Users", x: 355, y: 46, w: 170, h: 26, style: "actor" },
          { id: "ilb", label: "Internal LB", x: 560, y: 205, w: 100, h: 35, style: "infra" },
        ],
        links: [
          { from: "users", to: "pe", label: "via VPN", labelPos: 0.7 },
          { from: "pe", to: "ilb", label: "Private Link" },
          { from: "ilb", to: "hcpr" },
          { from: "w1", to: "pe", dashed: true },
          { from: "w2", to: "pe", dashed: true },
          { from: "w1", to: "kc", label: "reverse tunnel", dashed: true },
          { from: "w2", to: "kc", dashed: true },
        ],
      };
    };

    DIAGRAMS.agent_nodeport = function () {
      return {
        title: "Agent / Bare Metal — NodePort", height: 635,
        subtitle: "Publishing: All services → NodePort",
        networks: [
          { id: "app", y: 100, label: "Application Network", color: NC.ext },
          { id: "mgmt", y: 215, label: "Cluster Network", color: NC.mgmt },
          { id: "storage", y: 335, label: "Storage Network (etcd PVs — Ceph, NFS, iSCSI…)", color: NC.storage },
          { id: "dp", y: 425, label: "Data Plane Network", color: NC.dp },
          { id: "bmc", y: 535, label: "BMC / IPMI Network (provisioning)", color: NC.bmc, dashed: true },
        ],
        rows: [
          { network: "app", y: 120, items: [
            { id: "np", label: "NodePort", sub: ":30000+", w: 110, style: "infra" },
          ]},
          { network: "mgmt", y: 235, items: [
            { id: "kas", label: "API", sub: ":6443 · NP", w: 85, style: "cp", user: true },
            { id: "oauth", label: "OAuth", sub: ":443 · NP", w: 85, style: "cp", user: true },
            { id: "kc", label: "Konnectivity", sub: ":8091 · NP", w: 110, style: "cp" },
            { id: "ign", label: "Ignition", sub: ":8443 · NP", w: 85, style: "cp" },
            { id: "mgmtn", label: "Mgmt Nodes", sub: "etcd PVs", w: 100, h: 58, style: "def" },
            { id: "metal3", label: "Metal3", sub: "Ironic", w: 80, style: "def" },
          ]},
          { network: "dp", y: 445, items: [
            { id: "dpi", label: "DP Ingress", sub: ":80/:443", w: 110, style: "dp", user: true },
            { id: "w1", label: "Worker 1", w: 100, h: 58, style: "dp" },
            { id: "w2", label: "Worker 2", w: 100, h: 58, style: "dp" },
          ]},
        ],
        extraLegs: [
          { comp: "mgmtn", net: "storage" },
          { comp: "mgmtn", net: "bmc", dx: 27 },
          { comp: "mgmtn", net: "app" },
          { comp: "metal3", net: "bmc" },
          { comp: "w1", net: "bmc" },
          { comp: "w2", net: "bmc" },
          { comp: "w1", net: "storage" },
          { comp: "w2", net: "storage" },
          { comp: "w1", net: "app", dx: 98, startDy: -10 },
          { comp: "w2", net: "app", dx: -13, startDy: -10 },
          { comp: "dpi", net: "app", dx: -102 },
        ],
        floats: [
          { id: "users", label: "External Users", x: 370, y: 46, w: 140, h: 26, style: "actor" },
        ],
        links: [
          { from: "users", to: "np", label: "API clients" },
          { from: "np", to: "kas" },
          { from: "np", to: "oauth" },
          { from: "np", to: "kc" },
          { from: "np", to: "ign" },
          { from: "w1", to: "kc", label: "reverse tunnel", dashed: true },
          { from: "w2", to: "kc", dashed: true },
        ],
      };
    };

    DIAGRAMS.agent_loadbalancer = function () {
      return {
        title: "Agent / Bare Metal — LoadBalancer", height: 635,
        subtitle: "Publishing: API → LoadBalancer (MetalLB)  |  OAuth, Konnectivity, Ignition → Route",
        networks: [
          { id: "app", y: 100, label: "Application Network", color: NC.ext },
          { id: "mgmt", y: 215, label: "Cluster Network", color: NC.mgmt },
          { id: "storage", y: 335, label: "Storage Network (etcd PVs — Ceph, NFS, iSCSI…)", color: NC.storage },
          { id: "dp", y: 425, label: "Data Plane Network", color: NC.dp },
          { id: "bmc", y: 535, label: "BMC / IPMI Network (provisioning)", color: NC.bmc, dashed: true },
        ],
        rows: [
          { network: "app", y: 120, items: [
            { id: "mlb", label: "MetalLB", sub: "IPAddressPool", w: 120, style: "infra" },
            { id: "mgi", label: "Mgmt Ingress", sub: "OCP Router", w: 120, style: "infra" },
          ]},
          { network: "mgmt", y: 235, items: [
            { id: "kas", label: "API", sub: ":6443 · LB", w: 85, style: "cp", user: true },
            { id: "oauth", label: "OAuth", sub: ":443 · Route", w: 85, style: "cp", user: true },
            { id: "kc", label: "Konnectivity", sub: ":443 · Route", w: 110, style: "cp" },
            { id: "ign", label: "Ignition", sub: ":443 · Route", w: 85, style: "cp" },
            { id: "mgmtn", label: "Mgmt Nodes", sub: "etcd PVs", w: 100, h: 58, style: "def" },
            { id: "metal3", label: "Metal3", sub: "Ironic", w: 80, style: "def" },
          ]},
          { network: "dp", y: 445, items: [
            { id: "dpi", label: "DP Ingress", sub: ":80/:443", w: 110, style: "dp", user: true },
            { id: "w1", label: "Worker 1", w: 100, h: 58, style: "dp" },
            { id: "w2", label: "Worker 2", w: 100, h: 58, style: "dp" },
          ]},
        ],
        extraLegs: [
          { comp: "mgmtn", net: "storage" },
          { comp: "mgmtn", net: "bmc", dx: 27 },
          { comp: "mgmtn", net: "app" },
          { comp: "metal3", net: "bmc" },
          { comp: "w1", net: "bmc" },
          { comp: "w2", net: "bmc" },
          { comp: "w1", net: "storage" },
          { comp: "w2", net: "storage" },
          { comp: "w1", net: "app" },
          { comp: "w2", net: "app", dx: 101, startDy: -10 },
          { comp: "dpi", net: "app", dx: -102 },
        ],
        floats: [
          { id: "users", label: "External Users", x: 370, y: 46, w: 140, h: 26, style: "actor" },
        ],
        links: [
          { from: "users", to: "mlb", label: "API clients" },
          { from: "users", to: "mgi", label: "app users" },
          { from: "mlb", to: "kas" },
          { from: "mgi", to: "oauth" },
          { from: "mgi", to: "kc" },
          { from: "mgi", to: "ign" },
          { from: "w1", to: "kc", label: "reverse tunnel", dashed: true },
          { from: "w2", to: "kc", dashed: true },
        ],
      };
    };

    DIAGRAMS.kv_passthrough = function () {
      return {
        title: "KubeVirt — BaseDomainPassthrough", height: 540,
        subtitle: "Publishing: All services → Route (passthrough via mgmt ingress)",
        networks: [
          { id: "ext", y: 100, label: "External Network", color: NC.ext },
          { id: "mgmt", y: 250, label: "Management Cluster / Pod Network", color: NC.mgmt },
          { id: "dp", y: 410, label: "Data Plane — KubeVirt VMs (on pod network)", color: NC.dp },
        ],
        rows: [
          { network: "ext", y: 120, items: [
            { id: "mgi", label: "Mgmt Ingress", sub: "Wildcard DNS", w: 130, style: "infra" },
          ]},
          { network: "mgmt", y: 270, items: [
            { id: "kas", label: "API", sub: ":6443 · Route", w: 100, style: "cp", user: true },
            { id: "oauth", label: "OAuth", sub: ":443 · Route", w: 100, style: "cp", user: true },
            { id: "kc", label: "Konnectivity", sub: ":443 · Route", w: 120, style: "cp" },
            { id: "ign", label: "Ignition", sub: ":443 · Route", w: 100, style: "cp" },
          ]},
          { network: "dp", y: 430, items: [
            { id: "vm1", label: "VM 1", w: 100, h: 58, style: "dp" },
            { id: "vm2", label: "VM 2", w: 100, h: 58, style: "dp" },
            { id: "dpi", label: "DP Ingress", sub: ":443 only", w: 110, style: "dp", user: true },
          ]},
        ],
        floats: [
          { id: "users", label: "External Users", x: 370, y: 46, w: 140, h: 26, style: "actor" },
        ],
        links: [
          { from: "users", to: "mgi", label: "all traffic" },
          { from: "mgi", to: "kas" },
          { from: "mgi", to: "oauth" },
          { from: "mgi", to: "kc" },
          { from: "mgi", to: "ign" },
          { from: "mgi", to: "dpi", label: "passthrough", dashed: true, labelPos: 0.75 },
          { from: "vm1", to: "kc", label: "reverse tunnel", dashed: true, labelPos: 0.2 },
          { from: "vm2", to: "kc", dashed: true },
        ],
      };
    };

    DIAGRAMS.kv_custom = function () {
      return {
        title: "KubeVirt — Custom Domain + LB", height: 540,
        subtitle: "Publishing: API → LoadBalancer  |  OAuth, Konnectivity, Ignition → Route",
        networks: [
          { id: "ext", y: 100, label: "External Network", color: NC.ext },
          { id: "mgmt", y: 250, label: "Management Cluster / Pod Network", color: NC.mgmt },
          { id: "dp", y: 410, label: "Data Plane — KubeVirt VMs (on pod network)", color: NC.dp },
        ],
        rows: [
          { network: "ext", y: 120, items: [
            { id: "elb", label: "External LB", sub: "MetalLB / Cloud", w: 130, style: "infra" },
            { id: "mgi", label: "Mgmt Ingress", sub: "OCP Router", w: 120, style: "infra" },
            { id: "dns", label: "Custom DNS", sub: "*.apps.…", w: 110, style: "infra" },
          ]},
          { network: "mgmt", y: 270, items: [
            { id: "kas", label: "API", sub: ":6443 · LB", w: 100, style: "cp", user: true },
            { id: "oauth", label: "OAuth", sub: ":443 · Route", w: 100, style: "cp", user: true },
            { id: "kc", label: "Konnectivity", sub: ":443 · Route", w: 120, style: "cp" },
            { id: "ign", label: "Ignition", sub: ":443 · Route", w: 100, style: "cp" },
          ]},
          { network: "dp", y: 430, items: [
            { id: "vm1", label: "VM 1", w: 100, h: 58, style: "dp" },
            { id: "vm2", label: "VM 2", w: 100, h: 58, style: "dp" },
            { id: "dpi", label: "DP Ingress", sub: ":80/:443", w: 110, style: "dp", user: true },
          ]},
        ],
        floats: [
          { id: "users", label: "External Users", x: 370, y: 46, w: 140, h: 26, style: "actor" },
        ],
        links: [
          { from: "users", to: "elb", label: "API clients" },
          { from: "users", to: "dns", label: "app users" },
          { from: "elb", to: "kas" },
          { from: "mgi", to: "oauth" },
          { from: "mgi", to: "kc" },
          { from: "mgi", to: "ign" },
          { from: "dns", to: "dpi", label: "app traffic", dashed: true, labelPos: 0.75 },
          { from: "vm1", to: "kc", label: "reverse tunnel", dashed: true, labelPos: 0.2 },
          { from: "vm2", to: "kc", dashed: true },
        ],
      };
    };

    DIAGRAMS.kv_isolated = function () {
      return {
        title: "KubeVirt — Isolated (Multus)", height: 530,
        subtitle: "Publishing: User-managed (via Multus NADs)",
        networks: [
          { id: "mgmt", y: 100, label: "Management Cluster Network", color: NC.mgmt },
          { id: "multus", y: 260, label: "Multus Additional Networks (NADs)", color: NC.multus },
          { id: "dp", y: 400, label: "Data Plane — KubeVirt VMs (no pod network)", color: NC.dp },
        ],
        rows: [
          { network: "mgmt", y: 120, items: [
            { id: "kas", label: "API", sub: ":6443", w: 90, style: "cp", user: true },
            { id: "oauth", label: "OAuth", sub: ":443", w: 90, style: "cp", user: true },
            { id: "kc", label: "Konnectivity", sub: ":8091", w: 110, style: "cp" },
            { id: "ign", label: "Ignition", sub: ":443", w: 90, style: "cp" },
          ]},
          { network: "multus", y: 280, items: [
            { id: "nad1", label: "NAD 1", sub: "ns/net-1", w: 100, style: "infra" },
            { id: "nad2", label: "NAD 2", sub: "ns/net-2", w: 100, style: "infra" },
          ]},
          { network: "dp", y: 420, items: [
            { id: "vm1", label: "VM 1", sub: "no pod net", w: 100, h: 58, style: "dp" },
            { id: "vm2", label: "VM 2", sub: "no pod net", w: 100, h: 58, style: "dp" },
            { id: "dpi", label: "DP Ingress", sub: "user-managed", w: 120, style: "dp", user: true },
          ]},
        ],
        floats: [
          { id: "users", label: "External Users", x: 370, y: 46, w: 140, h: 26, style: "actor" },
        ],
        links: [
          { from: "users", to: "kas", label: "user-managed" },
          { from: "vm1", to: "nad1" },
          { from: "vm2", to: "nad1" },
          { from: "vm1", to: "nad2" },
          { from: "vm2", to: "nad2" },
          { from: "vm1", to: "kc", label: "reverse tunnel via NAD", dashed: true, labelPos: 0.15 },
          { from: "vm2", to: "kc", dashed: true },
        ],
      };
    };

    // ── Summaries ────────────────────────────────

    var SUMMARIES = {};

    SUMMARIES.aws_public_no = [
      { svc: "API", strategy: "LoadBalancer", port: "6443", detail: "External LB, public DNS — set hostname on all services (recommended)" },
      { svc: "OAuth", strategy: "Route", port: "443", detail: "Via management cluster ingress" },
      { svc: "Konnectivity", strategy: "Route", port: "443", detail: "Reverse tunnel: DP-initiated, enables CP→DP communication" },
      { svc: "Ignition", strategy: "Route", port: "443", detail: "Via management cluster ingress" },
      { svc: "DP Ingress", strategy: "—", port: "80/443", detail: "Guest cluster router on workers" },
    ];

    SUMMARIES.aws_public_yes = [
      { svc: "API", strategy: "Route", port: "6443", detail: "External LB → HCP Router — hostname required (External DNS creates Route53 records)" },
      { svc: "OAuth", strategy: "Route", port: "443", detail: "External LB → HCP Router — hostname required" },
      { svc: "Konnectivity", strategy: "Route", port: "443", detail: "Reverse tunnel: DP-initiated, enables CP→DP communication" },
      { svc: "Ignition", strategy: "Route", port: "443", detail: "External LB → HCP Router — hostname required" },
      { svc: "DP Ingress", strategy: "—", port: "80/443", detail: "Guest cluster router on workers" },
    ];

    SUMMARIES.aws_publicandprivate = [
      { svc: "API", strategy: "LoadBalancer", port: "6443", detail: "External LB (public access) + PrivateLink (data plane access)" },
      { svc: "OAuth", strategy: "Route", port: "443", detail: "Public: via mgmt ingress | DP: Workers → VPC Endpoint → Internal LB → HCP Router" },
      { svc: "Konnectivity", strategy: "Route", port: "443", detail: "Reverse tunnel: DP-initiated via PrivateLink, enables CP→DP communication" },
      { svc: "Ignition", strategy: "Route", port: "443", detail: "Workers → VPC Endpoint → Internal LB → HCP Router" },
      { svc: "DP Ingress", strategy: "—", port: "80/443", detail: "Guest cluster router on workers" },
    ];

    SUMMARIES.aws_private = [
      { svc: "API", strategy: "Route", port: "6443", detail: "Workers → VPC Endpoint → Internal LB → HCP Router" },
      { svc: "OAuth", strategy: "Route", port: "443", detail: "Workers → VPC Endpoint → Internal LB → HCP Router" },
      { svc: "Konnectivity", strategy: "Route", port: "443", detail: "Reverse tunnel: DP-initiated via PrivateLink, enables CP→DP communication" },
      { svc: "Ignition", strategy: "Route", port: "443", detail: "Workers → VPC Endpoint → Internal LB → HCP Router" },
      { svc: "DP Ingress", strategy: "—", port: "80/443", detail: "Guest cluster router on workers" },
    ];

    SUMMARIES.azure_public = [
      { svc: "API", strategy: "Route", port: "6443", detail: "Via management ingress, external DNS required" },
      { svc: "OAuth", strategy: "Route", port: "443", detail: "Via management ingress" },
      { svc: "Konnectivity", strategy: "Route", port: "443", detail: "Reverse tunnel: DP-initiated, enables CP→DP communication" },
      { svc: "Ignition", strategy: "Route", port: "443", detail: "Via management ingress" },
      { svc: "DP Ingress", strategy: "—", port: "80/443", detail: "Guest cluster router on workers" },
    ];

    SUMMARIES.azure_private = [
      { svc: "API", strategy: "Route", port: "6443", detail: "Workers → Private Endpoint → Internal LB → HCP Router" },
      { svc: "OAuth", strategy: "Route", port: "443", detail: "Workers → Private Endpoint → Internal LB → HCP Router" },
      { svc: "Konnectivity", strategy: "Route", port: "443", detail: "Reverse tunnel: DP-initiated via Private Link, enables CP→DP communication" },
      { svc: "Ignition", strategy: "Route", port: "443", detail: "Workers → Private Endpoint → Internal LB → HCP Router" },
      { svc: "DP Ingress", strategy: "—", port: "80/443", detail: "Guest cluster router on workers" },
    ];

    SUMMARIES.agent_nodeport = [
      { svc: "API", strategy: "NodePort", port: "6443→30000+", detail: "On management cluster nodes" },
      { svc: "OAuth", strategy: "NodePort", port: "443→30000+", detail: "On management cluster nodes" },
      { svc: "Konnectivity", strategy: "NodePort", port: "8091→30000+", detail: "Reverse tunnel: DP-initiated, enables CP→DP communication" },
      { svc: "Ignition", strategy: "NodePort", port: "8443→30000+", detail: "On management cluster nodes" },
      { svc: "DP Ingress", strategy: "—", port: "80/443", detail: "Guest cluster router on bare metal nodes" },
    ];

    SUMMARIES.agent_loadbalancer = [
      { svc: "API", strategy: "LoadBalancer", port: "6443", detail: "Via MetalLB (L2 Advertisement) — set hostname on all services (recommended)" },
      { svc: "OAuth", strategy: "Route", port: "443", detail: "Via management cluster ingress" },
      { svc: "Konnectivity", strategy: "Route", port: "443", detail: "Reverse tunnel: DP-initiated, enables CP→DP communication" },
      { svc: "Ignition", strategy: "Route", port: "443", detail: "Via management cluster ingress" },
      { svc: "DP Ingress", strategy: "—", port: "80/443", detail: "MetalLB or NodePort for external access" },
    ];

    SUMMARIES.kv_passthrough = [
      { svc: "API", strategy: "Route", port: "6443", detail: "Via management ingress (passthrough)" },
      { svc: "OAuth", strategy: "Route", port: "443", detail: "Via management ingress" },
      { svc: "Konnectivity", strategy: "Route", port: "443", detail: "Reverse tunnel: DP-initiated, enables CP→DP communication" },
      { svc: "Ignition", strategy: "Route", port: "443", detail: "Via management ingress" },
      { svc: "DP Ingress", strategy: "Passthrough", port: "443 only", detail: "Wildcard DNS via mgmt ingress, HTTPS only" },
    ];

    SUMMARIES.kv_custom = [
      { svc: "API", strategy: "LoadBalancer", port: "6443", detail: "External LB (MetalLB or cloud) — set hostname on all services (recommended)" },
      { svc: "OAuth", strategy: "Route", port: "443", detail: "Via management ingress" },
      { svc: "Konnectivity", strategy: "Route", port: "443", detail: "Reverse tunnel: DP-initiated, enables CP→DP communication" },
      { svc: "Ignition", strategy: "Route", port: "443", detail: "Via management ingress" },
      { svc: "DP Ingress", strategy: "LoadBalancer", port: "80/443", detail: "User-managed LB + custom wildcard DNS" },
    ];

    SUMMARIES.kv_isolated = [
      { svc: "API", strategy: "User-managed", port: "6443", detail: "Routing via Multus NAD configuration" },
      { svc: "OAuth", strategy: "User-managed", port: "443", detail: "Routing via Multus NAD configuration" },
      { svc: "Konnectivity", strategy: "User-managed", port: "8091", detail: "Reverse tunnel: DP-initiated via NAD, enables CP→DP communication" },
      { svc: "Ignition", strategy: "User-managed", port: "443", detail: "Routing via Multus NAD configuration" },
      { svc: "DP Ingress", strategy: "User-managed", port: "80/443", detail: "No mgmt cluster dependency" },
    ];

    // ── Init ─────────────────────────────────────

    render();
  }

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", init);
  } else {
    init();
  }
})();
