package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"html/template"

	"github.com/hashicorp/terraform-config-inspect/tfconfig"
)

func main() {
	provider := os.Args[1]

	if provider == "verify" {
		err := VerifyAll()
		if err != nil {
			panic(err)
		}
		return
	}

	path := fmt.Sprintf("terraform/system/%s/", provider)
	f, err := os.Create(filepath.Join(path, "telemetry.tf"))
	if err != nil {
		panic(err)
	}

	err = generateTelemetry(provider, f)
	if err != nil {
		panic(err)
	}
}

func VerifyAll() error {
	providers := []string{
		"aws",
		"azure",
		"do",
		"gcp",
	}

	for _, p := range providers {
		var b bytes.Buffer
		wr := bufio.NewWriter(&b)

		path := fmt.Sprintf("terraform/system/%s/", p)
		data, err := os.ReadFile(filepath.Join(path, "telemetry.tf"))
		if err != nil {
			return err
		}

		err = generateTelemetry(p, &b)
		if err != nil {
			return err
		}
		wr.Flush()

		if b.String() != string(data) {
			fmt.Printf("mistach found for %s\n", p)
			fmt.Println("+++++++++++++++++++++++")
			fmt.Println(b.String())
			fmt.Println("-----------------------")
			fmt.Println(string(data))
			fmt.Println("+++++++++++++++++++++++")
			return fmt.Errorf("mismatch")
		}
	}
	return nil
}

func generateTelemetry(provider string, w io.Writer) error {
	path := fmt.Sprintf("terraform/system/%s/", provider)
	module, diags := tfconfig.LoadModule(path)
	if diags.Err() != nil {
		return diags.Err()
	}

	varMap := map[string]string{}
	defaultMap := map[string]interface{}{}

	for _, v := range module.Variables {
		varMap[v.Name] = fmt.Sprintf("var.%s", v.Name)
		defaultMap[v.Name] = v.Default
	}

	tp := template.New("")

	tp1, err := tp.ParseFiles("cmd/telemetry-gen/telemetry.tf.tmpl")
	if err != nil {
		return err
	}

	return tp1.ExecuteTemplate(w, "telemetry", map[string]interface{}{
		"Provider":   provider,
		"VarMap":     varMap,
		"DefaultMap": defaultMap,
	})
}
