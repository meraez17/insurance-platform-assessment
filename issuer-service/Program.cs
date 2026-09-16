using System.Net;
using System.Net.Http.Json;
using System.Text.Json.Serialization;

var builder = WebApplication.CreateBuilder(args);
builder.Services.AddHttpClient<LegacyIssuerClient>(client =>
{
    client.BaseAddress = new Uri(builder.Configuration["LEGACY_ISSUER_URL"] ?? "http://localhost:8082/legacy/");
    client.Timeout = TimeSpan.FromSeconds(2);
});
var app = builder.Build();
app.MapGet("/health", () => Results.Ok(new { status = "ok" }));
app.MapPost("/policies", async (IssuePolicyRequest request, LegacyIssuerClient client, CancellationToken ct) =>
{
    if (string.IsNullOrWhiteSpace(request.ExternalRef)) return Results.BadRequest(new { code = "EXTERNAL_REF_REQUIRED" });
    var result = await client.Issue(request, ct);
    return result switch
    {
        IssueResult.Success value => Results.Ok(new { policyNumber = value.PolicyNumber, alreadyExisted = value.AlreadyExisted }),
        IssueResult.BusinessFailure value => Results.UnprocessableEntity(new { code = value.Code }),
        IssueResult.UnknownOutcome => Results.Json(new { code = "UNKNOWN_OUTCOME", reconciliationRequired = true }, statusCode: 503),
        _ => Results.StatusCode(503)
    };
});
app.MapGet("/policies/by-reference/{externalRef}", async (string externalRef, LegacyIssuerClient client, CancellationToken ct) =>
    await client.Find(externalRef, ct) is { } policy ? Results.Ok(new { policyNumber = policy }) : Results.NotFound());
app.Run();

public sealed record IssuePolicyRequest(string ExternalRef, string VehiclePlate, long PremiumCents);
public abstract record IssueResult
{
    public sealed record Success(string PolicyNumber, bool AlreadyExisted) : IssueResult;
    public sealed record BusinessFailure(string Code) : IssueResult;
    public sealed record UnknownOutcome : IssueResult;
    public sealed record TemporaryFailure : IssueResult;
}
public sealed class LegacyIssuerClient(HttpClient http)
{
    public async Task<IssueResult> Issue(IssuePolicyRequest request, CancellationToken ct)
    {
        try
        {
            var response = await http.PostAsJsonAsync("issue", new LegacyIssueRequest(request.ExternalRef, request.VehiclePlate, request.PremiumCents, request.PremiumCents), ct);
            if (response.StatusCode is HttpStatusCode.GatewayTimeout or HttpStatusCode.RequestTimeout) return new IssueResult.UnknownOutcome();
            if (!response.IsSuccessStatusCode) return new IssueResult.TemporaryFailure();
            var payload = await response.Content.ReadFromJsonAsync<LegacyIssueResponse>(cancellationToken: ct);
            if (payload is null) return new IssueResult.TemporaryFailure();
            if (!payload.Ok) return new IssueResult.BusinessFailure(payload.BusinessError ?? "LEGACY_BUSINESS_ERROR");
            if (string.IsNullOrWhiteSpace(payload.PolicyNumber)) return new IssueResult.TemporaryFailure();
            return new IssueResult.Success(payload.PolicyNumber, payload.Duplicate);
        }
        catch (TaskCanceledException) { return new IssueResult.UnknownOutcome(); }
        catch (HttpRequestException) { return new IssueResult.TemporaryFailure(); }
    }
    public async Task<string?> Find(string externalRef, CancellationToken ct)
    {
        var response = await http.GetAsync($"policies/{Uri.EscapeDataString(externalRef)}", ct);
        if (response.StatusCode == HttpStatusCode.NotFound) return null;
        response.EnsureSuccessStatusCode();
        return (await response.Content.ReadFromJsonAsync<LegacyLookupResponse>(cancellationToken: ct))?.PolicyNumber;
    }
}
public sealed record LegacyIssueRequest([property: JsonPropertyName("external_reference")] string ExternalReference, [property: JsonPropertyName("plate")] string Plate, [property: JsonPropertyName("premium")] long Premium, [property: JsonPropertyName("amount_duplicate")] long DuplicateAmount);
public sealed record LegacyIssueResponse([property: JsonPropertyName("ok")] bool Ok, [property: JsonPropertyName("policy_no")] string? PolicyNumber, [property: JsonPropertyName("business_error")] string? BusinessError, [property: JsonPropertyName("duplicate")] bool Duplicate);
public sealed record LegacyLookupResponse([property: JsonPropertyName("policy_no")] string? PolicyNumber);

