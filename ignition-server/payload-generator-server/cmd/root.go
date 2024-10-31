package cmd

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"go.uber.org/zap/zapcore"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

type Command struct {
	Bin  string
	Args []string
}

// Structure for POST request data
type RequestData struct {
	Mco Command
	Mce Command
	Mcs Command
}

type RunHTTPIgnitionPayloadGeneratorOpts struct {
	Namespace           string
	Image               string
	TokenSecret         string
	WorkDir             string
	FeatureGateManifest string
	Addr                string
	CertFile            string
	KeyFile             string
}

func NewRunHTTPIgnitionPayloadGenerator() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "payload-generator-server",
		Short: "Payload generator API server",
	}

	opts := RunHTTPIgnitionPayloadGeneratorOpts{
		Addr:     "0.0.0.0:9090",
		CertFile: "/var/run/secrets/ignition/serving-cert/tls.crt",
		KeyFile:  "/var/run/secrets/ignition/serving-cert/tls.key",
		WorkDir:  "/output",
	}

	cmd.Flags().StringVar(&opts.Addr, "addr", opts.Addr, "Listen address")
	//cmd.Flags().StringVar(&opts.CertFile, "cert-file", opts.CertFile, "Path to the serving cert")
	//cmd.Flags().StringVar(&opts.KeyFile, "key-file", opts.KeyFile, "Path to the serving key")
	cmd.Flags().StringVar(&opts.WorkDir, "work-dir", opts.WorkDir, "Directory in which to store transient working data")
	cmd.Flags().StringVar(&opts.FeatureGateManifest, "feature-gate-manifest", opts.FeatureGateManifest, "Path to a rendered featuregates.config.openshift.io/v1 file")

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		ctx, cancel := context.WithCancel(context.Background())
		sigs := make(chan os.Signal, 1)
		signal.Notify(sigs, syscall.SIGINT)
		go func() {
			<-sigs
			cancel()
		}()

		ctrl.SetLogger(zap.New(zap.UseDevMode(true), zap.JSONEncoder(func(o *zapcore.EncoderConfig) {
			o.EncodeTime = zapcore.RFC3339TimeEncoder
		})))
		return opts.Run(ctx)
	}

	return cmd
}

func (o *RunHTTPIgnitionPayloadGeneratorOpts) Run(ctx context.Context) error {
	http.HandleFunc("/data", handleData)
	if err := http.ListenAndServe(o.Addr, nil); err != nil {
		return err
	}

	return nil
}

// handleData handles POST requests to the /data endpoint
func handleData(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var requestData RequestData
	if err := json.NewDecoder(r.Body).Decode(&requestData); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	response := map[string]interface{}{
		"message": "Data received",
		"data":    requestData,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
