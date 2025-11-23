import sys
import os
import grpc
from concurrent import futures
from generated import embedding_pb2, embedding_pb2_grpc

# Import your embedding model here
# from transformers import AutoModel, AutoTokenizer
# import torch

class EmbeddingService(embedding_pb2_grpc.EmbeddingServiceServicer):
    def __init__(self):
        # Initialize your embedding model here
        # self.model = AutoModel.from_pretrained("sentence-transformers/all-MiniLM-L6-v2")
        # self.tokenizer = AutoTokenizer.from_pretrained("sentence-transformers/all-MiniLM-L6-v2")
        pass
    
    def GetEmbedding(self, request, context):
        """Generate embedding for a single prompt"""
        prompt = request.prompt
        
        # TODO: Call your embedding model here
        # embedding = self.generate_embedding(prompt)
        embedding = [0.1, 0.2, 0.3]  # Placeholder - replace with actual embedding
        
        return embedding_pb2.EmbeddingResponse(embedding=embedding)
    
    def GetEmbeddingBatch(self, request, context):
        """Generate embeddings for multiple prompts"""
        prompts = request.prompts
        embeddings = []
        
        for prompt in prompts:
            # TODO: Call your embedding model here
            embedding = [0.1, 0.2, 0.3]  # Placeholder
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