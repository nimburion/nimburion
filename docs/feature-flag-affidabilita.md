# Analisi affidabilità del sistema di feature flag

## Sintesi

Il sistema attuale è **solido per fallback sicuro e thread-safety di base**, ma presenta rischi di affidabilità in scenari reali (integrazione con provider esterni, governance delle chiavi, osservabilità e gestione eventi).

## Punti forti

- **Fallback sicuro esplicito**: in caso di errore provider o mismatch di tipo, il sistema usa il default dichiarato (`SourceSafeDefault`) invece di propagare failure.
- **Contratti tipizzati**: valutazioni separate per `bool`, `string`, `int` riducono ambiguità lato consumer.
- **Concorrenza protetta**: registry con `sync.RWMutex` per letture/scritture concorrenti.
- **Snapshot runtime**: disponibilità di `Snapshot()` per introspezione delle definizioni registrate.

## Rischi di affidabilità

### 1) Collisione chiavi tra tipi diversi

La stessa chiave può essere registrata in mappe diverse (`boolDefinitions`, `stringDefinitions`, `intDefinitions`) senza errore.
Impatto:

- potenziale comportamento incoerente tra moduli che valutano la stessa chiave con tipo diverso;
- in `Snapshot()`, l'ultima iterazione può sovrascrivere una voce con la stessa chiave.

### 2) Assenza di validazione input su registrazione

Non c'è validazione su chiavi vuote/duplicate (intra-tipo e inter-tipo) o descrizioni mancanti.
Impatto:

- errori di configurazione non intercettati al bootstrap;
- affidabilità operativa ridotta per mancanza di fail-fast.

### 3) Compatibilità tipi troppo rigida con provider esterni

Per `EvalInt` è accettato solo `int` puro. Provider esterni spesso ritornano `int64`/`float64` (es. deserializzazione JSON).
Impatto:

- fallback a default anche in presenza di valore semanticamente valido;
- rollout non prevedibile tra ambienti/provider diversi.

### 4) Osservabilità limitata sulla valutazione flag

L'errore è contenuto nell'`Evaluation.Error`, ma non c'è emissione nativa di metriche/eventi (error rate provider, percentuale fallback, flag missing).
Impatto:

- difficile rilevare regressioni di rollout in produzione;
- impossibile definire SLO/SLI affidabili sul comportamento dei flag.

### 5) Mancanze di observability sugli eventi runtime

Quando accadono eventi operativi (es. failure provider, fallback frequenti, mismatch di tipo, flag non registrata, cambio provider), manca una pipeline di osservabilità completa:

- **Nessun evento strutturato** dedicato (`feature_flag.evaluated`, `feature_flag.fallback_used`, `feature_flag.provider_error`, `feature_flag.type_mismatch`).
- **Nessuna correlazione cross-service**: manca propagation di trace/span attributes standard per key, source, outcome, environment, tenant.
- **Logging incompleto**: non ci sono log strutturati minimi per auditing e root-cause analysis.
- **No alerting semantics**: non esistono soglie operative (es. fallback rate > X% su una flag critica).
- **No audit trail dei cambiamenti**: assenza di eventi per registrazione/modifica provider/definizioni con actor e timestamp.

Impatto:

- incident response lenta perché gli eventi non sono interrogabili in modo uniforme;
- difficile distinguere tra "default atteso" e "fallback dovuto a failure";
- rischio di regressioni silenziose durante rollout progressivi.

### 6) Test coverage parziale sui casi limite

I test correnti coprono path principali (provider error, provider value, default, snapshot size), ma non coprono:

- collisioni inter-tipo;
- key non registrata per tutti i tipi;
- mismatch `int64 -> int`;
- concorrenza con `-race`;
- validazione telemetria/eventi emessi in condizioni di errore.

## Valutazione complessiva

- **Affidabile per safety di base** (non rompe il runtime in caso di errore provider).
- **Affidabilità medio-bassa per governance, integrazione enterprise e operations** senza hardening aggiuntivo.

## Raccomandazioni prioritarie

1. **Validazione forte in registrazione**
   - rifiutare chiavi vuote;
   - impedire collisioni cross-type;
   - opzionalmente rendere idempotente la ri-registrazione con stesso schema.
2. **Normalizzazione tipi provider**
   - in `EvalInt`, accettare anche `int64`/`float64` con controlli di overflow e conversione sicura.
3. **Osservabilità minima obbligatoria**
   - metriche: `flag_eval_total`, `flag_eval_fallback_total`, `flag_provider_error_total`, `flag_type_mismatch_total`, `flag_unregistered_total`;
   - label minime: `flag_key`, `kind`, `source`, `environment`, `service`, `tenant` (se disponibile);
   - tracing: span/event attributes standard per outcome e reason (`provider_error`, `type_mismatch`, `missing_flag`).
4. **Event model esplicito**
   - introdurre eventi strutturati versionati per: valutazione, fallback, errore provider, mismatch tipo, modifica provider/registry;
   - definire schema JSON stabile con correlation-id, actor, timestamp, severity, remediation hint;
   - integrare routing verso logging, metrics bridge e alerting policy.
5. **Test di robustezza**
   - aggiungere test edge-case e `go test -race` su package feature flag.
6. **Policy operativa**
   - introdurre catalogo centrale chiavi + ownership;
   - review obbligatoria per nuovi flag e sunset policy.

## Verdict finale

**Il sistema è sufficientemente affidabile per ambienti piccoli/controllati; per ambienti production multi-team è raccomandato un hardening (soprattutto su observability event-driven) prima di considerarlo "affidabile" in senso enterprise.**
