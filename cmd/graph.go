package cmd

import (
	"encoding/json"
	"html/template"
	"net/http"
	"sort"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/srl-wim/container-lab/clab"
)

const (
	templatePath = "/etc/containerlab/templates/d3js/index.html"
)

var srv string
var tmpl string

type graphTopo struct {
	Nodes []containerDetails `json:"nodes,omitempty"`
	Links []link             `json:"links,omitempty"`
}
type link struct {
	Source         string `json:"source,omitempty"`
	SourceEndpoint string `json:"source_endpoint,omitempty"`
	Target         string `json:"target,omitempty"`
	TargetEndpoint string `json:"target_endpoint,omitempty"`
}

type topoData struct {
	Name string
	Data template.JS
}

// graphCmd represents the graph command
var graphCmd = &cobra.Command{
	Use:   "graph",
	Short: "generate a topology graph",

	RunE: func(cmd *cobra.Command, args []string) error {
		opts := []clab.ClabOption{
			clab.WithDebug(debug),
			clab.WithTimeout(timeout),
			clab.WithTopoFile(topo),
		}
		c := clab.NewContainerLab(opts...)

		// Parse topology information
		if err := c.ParseTopology(); err != nil {
			return err
		}

		if srv == "" {
			if err := c.GenerateGraph(topo); err != nil {
				return err
			}
			return nil
		}
		gtopo := graphTopo{
			Nodes: make([]containerDetails, 0, len(c.Nodes)),
			Links: make([]link, 0, len(c.Links)),
		}
		for name, n := range c.Nodes {
			gtopo.Nodes = append(gtopo.Nodes, containerDetails{
				Name:  name,
				Kind:  n.Kind,
				Image: n.Image,
				Group: n.Group,
			})
		}
		sort.Slice(gtopo.Nodes, func(i, j int) bool {
			return gtopo.Nodes[i].Name < gtopo.Nodes[j].Name
		})
		for _, l := range c.Links {
			gtopo.Links = append(gtopo.Links, link{
				Source:         l.A.Node.ShortName,
				SourceEndpoint: l.A.EndpointName,
				Target:         l.B.Node.ShortName,
				TargetEndpoint: l.B.EndpointName,
			})
		}
		b, err := json.Marshal(gtopo)
		if err != nil {
			return err
		}
		log.Debug(string(b))
		topoD := topoData{
			Name: c.Config.Name,
			Data: template.JS(string(b)),
		}
		tmpl := template.Must(template.ParseFiles(tmpl))
		http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			tmpl.Execute(w, topoD)
		})

		log.Printf("Listening on %s...", srv)
		err = http.ListenAndServe(srv, nil)
		if err != nil {
			return err
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(graphCmd)
	graphCmd.Flags().StringVarP(&srv, "srv", "", "", "HTTP server address to view, customize and export your topology")
	graphCmd.Flags().StringVarP(&tmpl, "template", "", templatePath, "golang html template used to generate the graph")
}
