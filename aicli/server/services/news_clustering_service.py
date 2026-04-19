"""
News Clustering Service
Handles semantic embeddings and duplicate detection using SentenceTransformers.
"""
from sentence_transformers import SentenceTransformer, util

class NewsClusteringService:
    def __init__(self, threshold: float = 0.8):
        self.threshold = threshold
        # Load model lazily
        self._model = None

    def _get_model(self):
        if self._model is None:
            self._model = SentenceTransformer("all-MiniLM-L6-v2")
        return self._model

    def cluster_records(self, records: list[dict]) -> tuple[list[list[int]], int]:
        """
        Takes a list of records. Uses "news" key for text similarity.
        Returns a tuple of (clusters (list of record indices), count of duplicates found).
        """
        if not records:
            return [], 0

        model = self._get_model()
        news_texts = [r["news"] for r in records]
        
        # Compute embeddings and cosine similarities
        embeddings = model.encode(news_texts, convert_to_tensor=True)
        cos_scores = util.cos_sim(embeddings, embeddings)

        visited = set()
        clusters = []

        for i in range(len(records)):
            if i in visited:
                continue
            cluster = [i]
            visited.add(i)
            # Find all similar unvisited items
            for j in range(i + 1, len(records)):
                if j not in visited and cos_scores[i][j].item() >= self.threshold:
                    cluster.append(j)
                    visited.add(j)
            clusters.append(cluster)

        num_duplicates = sum(len(c) - 1 for c in clusters)
        return clusters, num_duplicates
