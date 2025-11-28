# DLI
Durable Layer Index

GoLang implementation for https://github.com/tkhdse/Durable-Layer-Index

![Local Dev Setup](images/dli-arch.png)


### Setup
Create .env file under /DLI/rb/:
```
cd rb
touch .env
```

Input following fields into .env file: 
```
# Embedding Service
EMBEDDING_SERVER_ADDR= # default: localhost:50051

# Pinecone Configuration
PINECONE_API_KEY=
PINECONE_INDEX=
PINECONE_REGION=
```


### Repository components:
Embedding Server 
DLI Logic (/DLI/rb/) 

Embedding Client (interface defined in /embedding/client.go)
Pinecone Client (interface defined in /vdb/pinecone.go)
