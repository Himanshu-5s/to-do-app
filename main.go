package main

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"
	"context"
	"os"
	"os/signal"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/thedevsaddam/renderer"
	mgo "gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)


var rnd *renderer.Render
var db *mgo.Database


const (
	hostName        string = "localhost:27017"
	dbName 						string = "demo_todo"
	collectionName  string = "todos"
	port            string = ":9000"
)

type (
	todoModel struct {
		ID        bson.ObjectId `bson:"_id", omitempty`
		Title 	 string        `bson:"title",`
		Completed bool          `bson:"completed",`
		CreatedAt time.Time     `bson:"created_at",`
	}

	todo struct {
		ID        string    `json:"id"`
		Title     string    `json:"title"`
		Completed bool      `json:"completed"`
		CreatedAt time.Time `json:"created_at"`
	}
)


func init(){
	rnd = renderer.New()
	session, err := mgo.Dial(hostName)
	checkError(err)
	session.SetMode(mgo.Monotonic, true)
	db = session.DB(dbName)
	}

	func homeHandler(w http.ResponseWriter, r *http.Request){
		err : rnd.Template(w, http.StatusOK, []string{"static/home.tpl"}, nil)
		checkError(err)
	}

	func getAllTodos(w http.ResponseWriter, r *http.Request){
		var todos []todoModel  //slice of type todoModel
		if err := db.C(collectionName).Find(bson.M{}).All(&todos); err!=nil{
		rnd.JSON(w, http.StatusProcessing, renderer.M{
			"message": "Error occured while fetching todos",
			"error": err,
		})
		return
	}
	todoList := []todo{}

	for _, t := range todos {
		todoList = append(todoList, todo{
			ID: t.ID.Hex(),
			Title: t.Title,
			Completed: t.Completed,
			CreatedAt: t.CreatedAt,
		})
	}
	rnd.JSON(w, http.StatusOK, renderer.M{
		"data" : todoList,
	})
}


func createTodo(w http.ResponseWriter, r *http.Request){
	var t todo
	if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
		rnd.JSON(w, http.StatusUnprocessableEntity, renderer.M{
			"message": "Invalid request data",
			"error": err,
		})
		return
	}
	if t.Title == "" {
		rnd.JSON(w, http.StatusUnprocessableEntity, renderer.M{
			"message": "Title is required",
		})
		return
}
tm := todoModel{
	ID: bson.NewObjectId(),
	Title: t.Title,
	Completed: t.Completed,
	CreatedAt: time.Now(),
}
if err := db.C(collectionName).Insert(&tm); err != nil {
	rnd.JSON(w, http.StatusProcessing, renderer.M{
		"message": "Error occured while saving todo",
		"error": err,
	})
	return
}

rnd.JSON(w, http.StatusCreated, renderer.M{
	"message": "Todo created successfully",
	"data": todo{
		ID: tm.ID.Hex(),
	},
})
}

func getTodo(w http.ResponseWriter, r *http.Request){
	todoID := chi.URLParam(r, "id")
	if !bson.IsObjectIdHex(todoID){
		rnd.JSON(w, http.StatusNotFound, renderer.M{
			"message": "Todo not found",
		})
		return
	}
	var t todoModel
	if err := db.C(collectionName).FindId(bson.ObjectIdHex(todoID)).One(&t); err != nil {
		rnd.JSON(w, http.StatusProcessing, renderer.M{
			"message": "Error occured while fetching todo",
			"error": err,
		})
		return
	}
	rnd.JSON(w, http.StatusOK, renderer.M{
		"data": todo{
			ID: t.ID.Hex(),
			Title: t.Title,
			Completed: t.Completed,
			CreatedAt: t.CreatedAt,
		},
	})
}

func deleteTodo(w http.ResponseWriter, r *http.Request){
	id := strings.TrimSpace(chi.URLParam(r, "id"))

	if !bson.IsObjectIdHex(id){
		rnd.JSON(w, http.StatusNotFound, renderer.M{
			"message": "Todo not found",
		})
		return
	}
	if err := db.C(collectionName).RemoveId(bson.ObjectIdHex(id)); err != nil {
		rnd.JSON(w, http.StatusProcessing, renderer.M{
			"message": "Error occured while deleting todo",
			"error": err,
		})
		return
	}
	rnd.JSON(w, http.StatusOK, renderer.M{
		"message": "Todo deleted successfully",
	})
}

func updateTodo(w http.ResponseWriter, r *http.Request){
	id := strings.TrimSpace(chi.URLParam(r, "id"))
	if !bson.IsObjectIdHex(id){
		rnd.JSON(w, http.StatusNotFound, renderer.M{
			"message": "Todo not found",
		})
		return
	}
	var t todo
	if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
		rnd.JSON(w, http.StatusUnprocessableEntity, renderer.M{
			"message": "Invalid request data",
			"error": err,
		})
		return
	}
	if t.Title == "" {
		rnd.JSON(w, http.StatusUnprocessableEntity, renderer.M{
			"message": "Title is required",
		})
		return
	}
	if err := db.C(collectionName).UpdateId(bson.ObjectIdHex(id), bson.M{"$set": bson.M{
		"title": t.Title,
		"completed": t.Completed,
	}}); err != nil {
		rnd.JSON(w, http.StatusProcessing, renderer.M{
			"message": "Error occured while updating todo",
			"error": err,
		})
		return
	}
	rnd.JSON(w, http.StatusOK, renderer.M{
		"message": "Todo updated successfully",
	})
}

func main(){
	// stopping the server gracefully
	stopChan := make(chan os.Signal)
	signal.Notify(stopChan, os.Interrupt)
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Get("/", homeHandler)
	r.Mount("/todos", todoHandlers())

	srv := &http.Server{
		Addr: port,
		Handler: r,
		ReadTimeout: 60 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout: 120 * time.Second,
	}
	go func(){
		log.Printf("Server is running on %s", port)
		if err := srv.ListenAndServe(); err != nil {
			log.Printf("listen: %s\n", err)
		}
	}()

	<-stopChan
	log.Println("Shutting down the server...")
	ctx, cancel := context.WithTimeout(context.Background(), 5 * time.Second)
	srv.Shutdown(ctx)
	defer cancel(
		log.Println("Server gracefully stopped"
	)
}

func todoHandlers() http.Handler {
	rg := chi.NewRouter()
	rg.Group(func(r chi.Router){
		r.Get("/", getAllTodos)
		r.Post("/", createTodo)
		r.Get("/{id}", getTodo)
		r.Put("/{id}", updateTodo)
		r.Delete("/{id}", deleteTodo)
	})
	return rg
}




func checkError(err error){
	if err != nil {
		log.Fatalf("Error: %v", err)
	}
}