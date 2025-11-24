import sys
import os
import grpc
from concurrent import futures
from generated import embedding_pb2, embedding_pb2_grpc
from langchain_huggingface import HuggingFaceEmbeddings

class EmbeddingService(embedding_pb2_grpc.EmbeddingServiceServicer):
    def __init__(self):
        self.embedder = HuggingFaceEmbeddings(
            model_name="sentence-transformers/all-MiniLM-L6-v2"
        )
    
    def GetEmbedding(self, request, context):
        """Generate embedding for a single prompt"""
        prompt = request.prompt
        embedding = self.embedder.embed_query(prompt)
        return embedding_pb2.EmbeddingResponse(embedding=embedding)
    
    def GetEmbeddingBatch(self, request, context):
        """Generate embeddings for multiple prompts"""
        prompts = request.prompts
        embeddings = []
        
        for prompt in prompts:
            embedding = self.embedder.embed_query(prompt)
            embeddings.append(embedding_pb2.EmbeddingResponse(embedding=embedding))
        
        return embedding_pb2.EmbeddingBatchResponse(embeddings=embeddings)

def serve():
    server = grpc.server(futures.ThreadPoolExecutor(max_workers=10))
    embedding_pb2_grpc.add_EmbeddingServiceServicer_to_server(
        EmbeddingService(), server
    )
    
    # Listen on port 50051
    server.add_insecure_port('[::]:50051')
    server.start()
    print("Embedding server started on port 50051")
    server.wait_for_termination()

if __name__ == '__main__':
    serve()