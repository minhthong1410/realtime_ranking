### **Problem Statement: Build a Microservice for Real-time Ranking**

#### **Problem Description**

You need to develop a microservice that handles **real-time ranking** for a video system. Each **entity** (such as a user, creator, or channel) has multiple videos, and each video has a **score**. The system should rank videos based on their **score** in **descending order**.

The **score** of each video is dynamic and depends on user interactions, such as: **Views, Likes, Comments, Shares, Watch time**

---

### **Requirements**

1. **Real-time ranking**: The ranking should be updated dynamically as new interactions occur.  
2. **Scalability**: The system should handle a large number of videos and interactions.  
3. **Low latency**: Queries for top-ranked videos should be fast.  
4. **Per-user ranking**: The ranking may be personalized for each user based on their preferences or history.  
5. **Efficient storage and retrieval**: Use an efficient data structure to update and fetch rankings quickly.  
6. **Swagger API Documentation**: The microservice must expose RESTful APIs documented using **Swagger (OpenAPI)**.  
7. **System Architecture Diagram**: You must provide a high-level **architecture diagram** explaining how different components interact within the microservice.

---

### **Technical Constraints**

* The microservice should expose APIs to:  
  * **Update video score** when a new interaction occurs.  
  * **Retrieve top-ranked videos** globally or per user.  
* The system should be implemented using **Go**.  
* Use a **real-time database** or **caching layer** to handle ranking efficiently.

---

### **Deliverables**

* A **fully functional microservice** with:  
  * RESTful APIs for updating and fetching rankings.  
  * **Swagger (OpenAPI) documentation** for all API endpoints.  
  * A **README** explaining how to deploy and use the service.  
  * **Unit tests** to verify core functionality.  
  * **A system architecture diagram** illustrating how the microservice works.

---

### **Note**

You have **3 days** to implement features that showcase your skills and abilities. You **don’t need to implement everything**—just focus on demonstrating your expertise.

**You are free to set your own rules in the application** (if specific details are not provided above).