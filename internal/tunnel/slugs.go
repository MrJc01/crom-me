package tunnel

import (
	"fmt"
	"math/rand"
	"sync"

	"github.com/gorilla/websocket"
)

// Adjetivos para geração de slugs amigáveis
var adjectives = []string{
	"agil", "bravo", "calmo", "doce", "forte",
	"gentil", "habil", "leve", "nobre", "rapido",
	"sabio", "tenaz", "veloz", "astuto", "fiel",
	"ativo", "belo", "claro", "digno", "exato",
	"firme", "grande", "justo", "livre", "manso",
	"nitido", "puro", "raro", "sutil", "vivo",
}

// Animais para geração de slugs amigáveis
var animals = []string{
	"tigre", "lobo", "falcao", "coruja", "gato",
	"urso", "leao", "cobra", "aguia", "puma",
	"raposa", "cervo", "condor", "javali", "lince",
	"tubarao", "polvo", "corvo", "pantera", "gaviao",
	"touro", "cisne", "coiote", "arara", "orca",
	"colibri", "lagarto", "furão", "castor", "faisao",
}

// slugMu protege a geração concorrente de slugs
var slugMu sync.Mutex

// GenerateSlug cria um slug amigável no formato "adjetivo-animal"
func GenerateSlug() string {
	adj := adjectives[rand.Intn(len(adjectives))]
	animal := animals[rand.Intn(len(animals))]
	return adj + "-" + animal
}

// GenerateUniqueSlug gera um slug que não colide com os existentes no mapa de conexões.
// Usa retry com até 100 tentativas antes de adicionar sufixo numérico.
func GenerateUniqueSlug(existing map[string]*websocket.Conn) string {
	slugMu.Lock()
	defer slugMu.Unlock()

	for i := 0; i < 100; i++ {
		slug := GenerateSlug()
		if _, exists := existing[slug]; !exists {
			return slug
		}
	}

	// Fallback: slug + sufixo numérico aleatório (probabilidade ínfima)
	for {
		slug := GenerateSlug() + fmt.Sprintf("-%d", rand.Intn(9999))
		if _, exists := existing[slug]; !exists {
			return slug
		}
	}
}
