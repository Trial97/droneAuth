package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"

	"github.com/drone/drone-go/drone"
	"github.com/drone/drone-go/plugin/validator"

	"github.com/99designs/httpsignatures-go"
)

var (
	bind = flag.String("endpoint", ":3000", "the endpoint to lisen at")
	// generated with:
	// openssl rand -hex 16
	secret = flag.String("secret", "", "the RPC secret(can be generated with 'openssl rand -hex 16')")
	debug  = flag.Bool("debug", false, "set log level to debug")
	file   = flag.String("cfg", "users.json", "the path to the users rights")
)

func main() {
	flag.Parse()
	if *secret == "" {
		log.Fatalln("missing secret key")
	}
	var users map[string]string
	if f, err := os.Open(*file); err != nil {
		log.Fatal(err)
	} else if err = json.NewDecoder(f).Decode(&users); err != nil {
		log.Fatal(err)
	}

	log.Printf("server listening on address %s", *bind)

	http.Handle("/", &handler{
		secret: *secret,
		users:  users,
		debug:  *debug,
	})
	log.Fatal(http.ListenAndServe(*bind, nil))
}

type handler struct {
	secret string
	users  map[string]string

	debug bool
}

func (p *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	signature, err := httpsignatures.FromRequest(r)
	if err != nil {
		if p.debug {
			log.Printf("validator: invalid or missing signature in http.Request")
		}
		http.Error(w, "Invalid or Missing Signature", 400)
		return
	}
	if !signature.IsValid(p.secret, r) {
		if p.debug {
			log.Printf("validator: invalid signature in http.Request")
		}
		http.Error(w, "Invalid Signature", 400)
		return
	}

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		if p.debug {
			log.Printf("validator: cannot read http.Request body")
		}
		w.WriteHeader(400)
		return
	}

	req := &validator.Request{}
	err = json.Unmarshal(body, req)
	if err != nil {
		if p.debug {
			log.Printf("validator: cannot unmarshal http.Request body")
		}
		http.Error(w, "Invalid Input", 400)
		return
	}

	if p.debug {
		log.Printf(string(body))
	}

	var code int
	var errMsg string
	switch p.users[req.Build.Author] {
	case "autoBuild":
		w.WriteHeader(http.StatusNoContent)
		return
	case "noBuild":
		code = http.StatusBadRequest
		errMsg = fmt.Sprintf("The user:<%s> has no build rights", req.Build.Author)
	case "skipBuild":
		code = 498
		errMsg = fmt.Sprintf("The user:<%s> builds are skiped", req.Build.Author)
	case "manualBuild":
		code = 498
		errMsg = fmt.Sprintf("The user:<%s> builds needs to be verified", req.Build.Author)
	default:
		code = 498
		errMsg = fmt.Sprintf("The user:<%s> builds needs to be verified", req.Build.Author)
	}
	if p.debug {
		log.Printf(errMsg)
	}
	out, _ := json.Marshal(&drone.Error{
		Code:    code,
		Message: errMsg,
	})
	w.WriteHeader(code)
	w.Write(out)
}
