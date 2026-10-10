import java.io.*;
import java.nio.charset.StandardCharsets;
import java.util.*;
import java.net.*;
import java.nio.file.*;
import org.mule.weave.v2.runtime.BindingValue;
import org.mule.weave.v2.runtime.ScriptingBindings;
import org.mule.weave.v2.runtime.api.*;
import org.mule.weave.v2.sdk.ClassLoaderWeaveResourceResolver;
import org.mule.weave.v2.model.ServiceManager;
import org.mule.weave.v2.model.service.SecurityManagerService;

/** Owned host adapter. The engine evaluates the supplied source without rewriting it. */
public final class Adapter {
    private static final class HostFailure extends IOException {
        final String kind;
        HostFailure(String kind, String message) { super(message); this.kind = kind; }
    }
    private static final int MAX_FIELD = 16 * 1024 * 1024;
    private static byte[] readBytes(DataInputStream in) throws IOException {
        int size = in.readInt();
        if (size < 0 || size > MAX_FIELD) throw new HostFailure("protocol", "invalid request field length");
        byte[] value = in.readNBytes(size);
        if (value.length != size) throw new HostFailure("protocol", "truncated request");
        return value;
    }
    private static String readString(DataInputStream in) throws IOException {
        return new String(readBytes(in), StandardCharsets.UTF_8);
    }
    private static void writeBytes(DataOutputStream out, byte[] value) throws IOException {
        out.writeInt(value.length);
        out.write(value);
    }
    private static void writeString(DataOutputStream out, String value) throws IOException {
        writeBytes(out, value.getBytes(StandardCharsets.UTF_8));
    }
    public static void main(String[] args) throws Exception {
        DataInputStream in = new DataInputStream(System.in);
        // Reserve stdout for the protocol, including if the engine logs to System.out.
        DataOutputStream out = new DataOutputStream(System.out);
        System.setOut(System.err);
        String phase = "protocol";
        try {
            if (in.readInt() != 5) throw new HostFailure("protocol", "unsupported protocol version");
            String name = readString(in);
            String source = readString(in);
            String apiVersion = readString(in); // Retained by host request identity; no inferred engine parity.
            int count = in.readInt();
            if (count < 0 || count > 1024) throw new HostFailure("protocol", "invalid input count");
            String[] names = new String[count];
            ScriptingBindings bindings = new ScriptingBindings();
            for (int i = 0; i < count; i++) {
                names[i] = readString(in);
                String mimeType = readString(in);
                int mode=in.readUnsignedByte();
                if(mode>1)throw new HostFailure("protocol","invalid typed input mode");
                byte[] value = readBytes(in);
                bindings.addBinding(names[i], new BindingValue(new OwnedApexFormat.BoundInput(value,mode==1), mimeType.isEmpty() ? scala.Option.empty() : scala.Option.apply(mimeType),
                    scala.collection.immutable.Map$.MODULE$.empty(), StandardCharsets.UTF_8));
            }
            int moduleCount = in.readInt();
            if (moduleCount < 0 || moduleCount > 1024) throw new HostFailure("protocol", "invalid module count");
            Map<String, org.mule.weave.v2.sdk.WeaveResource> modules = new HashMap<>();
            for (int i = 0; i < moduleCount; i++) {
                String binding = readString(in);
                String moduleName = readString(in);
                String namespace = readString(in);
                String moduleApiVersion = readString(in); // Preserved in host identity; no inferred version parity.
                String moduleSource = readString(in);
                if (binding.isEmpty() || moduleName.isEmpty() || modules.containsKey(binding)) throw new HostFailure("protocol", "invalid module binding");
                modules.put(binding, new org.mule.weave.v2.sdk.WeaveResource() {
                    public String url() { return "glade-module:" + binding; }
                    public String content() { return moduleSource; }
                });
            }
            if (in.read() != -1) throw new HostFailure("protocol", "trailing request bytes");
            phase = "compile";
            Set<Path> trustedJars = new HashSet<>();
            for (String path : System.getProperty("java.class.path").split(java.util.regex.Pattern.quote(File.pathSeparator))) {
                Path candidate = Path.of(path);
                if (path.endsWith(".jar") && Files.isRegularFile(candidate)) trustedJars.add(candidate.toRealPath());
            }
            var builtinResolver = ClassLoaderWeaveResourceResolver.noContextClassloader();
            org.mule.weave.v2.sdk.WeaveResourceResolver resolver = identifier -> {
                // Explicit caller-selected resources are in-memory source, never
                // paths to search on the host. Custom loaders (including Java)
                // cannot obtain a resource through this project-module bridge.
                if (identifier.loader().isDefined()) return scala.Option.empty();
                var module = modules.get(identifier.fullQualifiedName());
                if (module != null) return scala.Option.apply(module);
                if (!org.mule.weave.v2.parser.ast.variables.NameIdentifier.isDataWeaveSdkFile(identifier)) return scala.Option.empty();
                var resource = builtinResolver.resolve(identifier);
                if (resource.isEmpty()) return resource;
                // A builtin-looking name alone is insufficient: an adapter or
                // project directory must not inject dw/OwnedCustom.dwl.
                try {
                    URL url = new URL(resource.get().url());
                    if (!url.getProtocol().equals("jar")) return scala.Option.empty();
                    JarURLConnection connection = (JarURLConnection)url.openConnection();
                    URL origin = connection.getJarFileURL();
                    if (!origin.getProtocol().equals("file") || !trustedJars.contains(Path.of(origin.toURI()).toRealPath())) return scala.Option.empty();
                    return resource;
                } catch (Exception invalidOrigin) { return scala.Option.empty(); }
            };
            var factory = DWModuleComponentsFactory.createSimpleDWModuleComponentsFactoryBuilder()
                .withWeaveResourceResolver(resolver).enableSPIModuleLoader(false).build();
            var engine = DWScriptingEngine.builder().withDWModuleComponentsFactory(factory).build();
            // Apex executes the resource body as an anonymous DW compilation unit.
            // The request retains its resource name/version for host identity;
            // engine diagnostics refer to the source unit, not that resource key.
            var script = engine.compileDWScript("anonymous", source, names);
            script.setMaxTime(10000);
            SecurityManagerService deny = (privilege, values) -> false;
            scala.collection.immutable.Map<Class<?>, Object> services = scala.collection.immutable.Map$.MODULE$.empty();
            services = services.$plus(new scala.Tuple2<Class<?>, Object>(SecurityManagerService.class, deny));
            phase = "execute";
            DWResult result = script.writeDWResult(bindings, ServiceManager.apply(services));
            if(OwnedApexFormat.hostFailure()!=null)throw OwnedApexFormat.hostFailure();
            byte[] data;
            try (InputStream content = (InputStream)result.getContent()) {
                if(content instanceof OwnedApexFormat.BoundOutput typed && typed.failure!=null)throw typed.failure;
                data = content.readNBytes(MAX_FIELD + 1);
                if (data.length > MAX_FIELD) throw new HostFailure("output-limit", "result exceeds adapter byte limit");
            }
            if(OwnedApexFormat.hostFailure()!=null)throw OwnedApexFormat.hostFailure();
            out.writeByte(0);
            writeString(out, result.getMimeType());
            writeString(out, result.getCharset().name());
            writeBytes(out, data);
        } catch (HostFailure error) {
            out.writeByte(2);
            writeString(out, error.kind);
            writeString(out, error.getMessage());
        } catch (Exception error) {
            OwnedApexFormat.HostFailure host=OwnedApexFormat.hostFailure();
            Throwable cause=error;
            for(int depth=0;cause!=null&&depth<64;depth++,cause=cause.getCause()){if(cause instanceof OwnedApexFormat.HostFailure found){host=found;break;}}
            if(host!=null){out.writeByte(2);writeString(out,host.kind);writeString(out,host.getMessage());}
            else {out.writeByte(1);writeString(out,phase);writeString(out,error.getClass().getName());writeString(out,Objects.toString(error.getMessage(), ""));}
        }
        out.flush();
    }
}
