using Neo;

namespace ServiceLayer.Oracle
{
    /// <summary>Oracle request payload from user contract</summary>
    public class OracleRequestPayload
    {
        public string Url;        // URL to fetch
        public string Method;     // HTTP method (GET, POST)
        public string Headers;    // JSON-encoded headers
        public string JsonPath;   // JSONPath to extract from response
        public string Body;       // Request body for POST
    }

    /// <summary>Stored Oracle request</summary>
    public class OracleStoredRequest
    {
        public string Url;
        public string Method;
        public string Headers;
        public string JsonPath;
        public UInt160 UserContract;
    }
}
