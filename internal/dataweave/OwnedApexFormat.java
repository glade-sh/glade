import java.io.*;
import java.nio.charset.StandardCharsets;
import java.util.*;
import java.util.concurrent.atomic.AtomicReference;
import org.mule.weave.v2.module.*;
import org.mule.weave.v2.module.option.Settings;
import org.mule.weave.v2.module.reader.SourceProvider;
import org.mule.weave.v2.module.writer.Writer;
import org.mule.weave.v2.model.EvaluationContext;
import org.mule.weave.v2.model.values.*;
import org.mule.weave.v2.model.structure.*;
import org.mule.weave.v2.model.structure.schema.*;
import org.mule.weave.v2.parser.module.MimeType;

/** Owned bounded Apex value bridge using the engine's public data-format API. */
public class OwnedApexFormat implements DataFormat<Settings,Settings> {
    private static final int LIMIT=16*1024*1024, NODES=100000;
    public static final class BoundInput extends ByteArrayInputStream {
        final boolean typed;
        public BoundInput(byte[] data,boolean typed){super(data);this.typed=typed;}
    }
    public static final class BoundOutput extends ByteArrayInputStream {
        final HostFailure failure;
        BoundOutput(byte[] data,HostFailure failure){super(data);this.failure=failure;}
    }
    // This reader-owned carrier retains queried child-relationship provenance.
    // Ordinary DW arrays and arrays constructed by transformations are distinct.
    private static final class QueryRelationship extends ArrayValue.MaterializedArrayValue {
        QueryRelationship(ArrayValue values,EvaluationContext ctx,String inputName){
            super(values.evaluate(ctx),()->org.mule.weave.v2.parser.location.Location.apply(inputName),scala.Option.empty());
        }
    }
    private static final class OutputBuffer extends ByteArrayOutputStream {
        public synchronized void write(int value){if(count>=LIMIT)throw new HostFailure("output-limit","typed output exceeds byte limit");super.write(value);}
        public synchronized void write(byte[] value,int offset,int length){if(length>LIMIT-count)throw new HostFailure("output-limit","typed output exceeds byte limit");super.write(value,offset,length);}
        void countAt(int position,int value){buf[position]=(byte)(value>>>24);buf[position+1]=(byte)(value>>>16);buf[position+2]=(byte)(value>>>8);buf[position+3]=(byte)value;}
    }
    // Each adapter JVM serves one request. Preserve host failures even when the
    // engine wraps or catches a nested writer failure inside a DW expression.
    private static final AtomicReference<HostFailure> hostFailure=new AtomicReference<>();
    static HostFailure hostFailure(){return hostFailure.get();}
    public static final class HostFailure extends RuntimeException {
        final String kind;
        HostFailure(String kind,String message){super(message);this.kind=kind;hostFailure.compareAndSet(null,this);}
    }
    private final org.mule.weave.v2.module.core.textplain.TextPlainDataFormat defaults=new org.mule.weave.v2.module.core.textplain.TextPlainDataFormat();
    public String name(){return "apex";}
    public MimeType defaultMimeType(){return MimeType.fromSimpleString("application/apex");}
    public scala.collection.Seq<MimeType> acceptedMimeTypes(){return scala.collection.JavaConverters.asScalaBufferConverter(List.of(defaultMimeType())).asScala().toSeq();}
    public scala.collection.Seq<String> fileExtensions(){return scala.collection.JavaConverters.asScalaBufferConverter(List.of("apex")).asScala().toSeq();}
    protected boolean supportsOutput(){return true;}
    public static final class JavaInputFormat extends OwnedApexFormat {
        public String name(){return "java";}
        public MimeType defaultMimeType(){return MimeType.APPLICATION_JAVA();}
        public scala.collection.Seq<String> fileExtensions(){return scala.collection.JavaConverters.asScalaBufferConverter(List.of("java")).asScala().toSeq();}
        protected boolean supportsOutput(){return false;}
    }
    public Settings readerSettings(){return defaults.readerSettings();}
    public Settings writerSettings(){return defaults.readerSettings();}
    public org.mule.weave.v2.module.reader.Reader reader(SourceProvider source,EvaluationContext context){
        return new org.mule.weave.v2.module.reader.Reader(){
            public Settings settings(){return readerSettings();}
            public scala.Option<DataFormat<?,?>> dataFormat(){return scala.Option.apply(OwnedApexFormat.this);}
            public Value<?> doRead(String name){
                InputStream stream=source.asInputStream(context);
                Object underlying=source.underling();
                BoundInput input=underlying instanceof BoundInput b?b:stream instanceof BoundInput b?b:null;
                if(input==null)throw new HostFailure("typed-input","Apex reader requires an explicitly bound host input");
                try {
                    if(!input.typed){byte[] bytes=input.readNBytes(LIMIT+1);if(bytes.length>LIMIT)throw new HostFailure("input-limit","Apex string input exceeds byte limit");return StringValue.apply(new String(bytes,StandardCharsets.UTF_8));}
                    DataInputStream in=new DataInputStream(input);
                    Value<?> value=OwnedApexFormat.read(in,context,0,new int[]{0},name);
                    if(in.read()!=-1)throw new HostFailure("protocol","trailing typed input bytes");
                    return value;
                }catch(IOException error){throw new HostFailure("protocol","invalid typed input: "+error.getMessage());}
            }
        };
    }
    public Writer writer(scala.Option<Object> target,MimeType mime,EvaluationContext context){
        return new Writer(){
            private byte[] result=new byte[0];
            private HostFailure failure;
            public Settings settings(){return writerSettings();}
            public scala.Option<DataFormat<?,?>> dataFormat(){return scala.Option.apply(OwnedApexFormat.this);}
            public void doWriteValue(Value<?> value,EvaluationContext ctx){
                OutputBuffer bytes=new OutputBuffer();
                try {
                    if(!supportsOutput())throw new HostFailure("unsupported-output-format","DataWeave output format application/java is not supported");
                    write(new DataOutputStream(bytes),value,ctx,0,new int[]{0},bytes);result=bytes.toByteArray();
                }
                catch(HostFailure error){failure=error;}
                catch(IOException error){failure=new HostFailure("typed-output",error.getMessage());}
            }
            public Object result(){return new BoundOutput(result,failure);}
            public void close(){}
        };
    }
    private static String text(DataInputStream in)throws IOException{
        int size=in.readInt();if(size<0||size>LIMIT)throw new HostFailure("protocol","invalid typed field length");
        byte[] data=in.readNBytes(size);if(data.length!=size)throw new EOFException();return new String(data,StandardCharsets.UTF_8);
    }
    private static void text(DataOutputStream out,String value)throws IOException{
        byte[] data=value.getBytes(StandardCharsets.UTF_8);if(data.length>LIMIT)throw new HostFailure("output-limit","typed field exceeds byte limit");out.writeInt(data.length);out.write(data);
    }
    private static int count(DataInputStream in)throws IOException{int count=in.readInt();if(count<0||count>NODES)throw new HostFailure("protocol","invalid typed count");return count;}
    private static void structure(int depth,int[] nodes){if(depth>64||++nodes[0]>NODES)throw new HostFailure("structure-limit","typed value exceeds structure limit");}
    @SuppressWarnings({"rawtypes","unchecked"})
    private static Value<?> read(DataInputStream in,EvaluationContext ctx,int depth,int[] nodes,String inputName)throws IOException{
        structure(depth,nodes);int kind=in.readUnsignedByte();String type=text(in);
        switch(kind){
            case 0:return NullValue$.MODULE$;
            case 1:return StringValue.apply(text(in));
            case 2:return NumberValue.apply(Integer.parseInt(text(in)));
            case 8:return NumberValue.apply(org.mule.weave.v2.model.values.math.Number.apply(text(in)));
            case 9:return LocalDateValue.apply(java.time.LocalDate.parse(text(in)));
            case 10:return DateTimeValue.apply(java.time.ZonedDateTime.parse(text(in)));
            case 11:return LocalTimeValue.apply(java.time.LocalTime.parse(text(in)));
            case 12:return BinaryValue.apply(Base64.getDecoder().decode(text(in)));
            case 3:return NumberValue.apply(org.mule.weave.v2.model.values.math.Number.apply(text(in)));
            case 4:int flag=in.readUnsignedByte();if(flag>1)throw new HostFailure("protocol","invalid typed Boolean");return flag==1?BooleanValue.TRUE_BOOL():BooleanValue.FALSE_BOOL();
            case 5:case 13:{int n=count(in);Value<?>[] values=new Value<?>[n];for(int i=0;i<n;i++)values[i]=read(in,ctx,depth+1,nodes,inputName);ArrayValue array=ArrayValue.apply(values);return kind==13?new QueryRelationship(array,ctx,inputName):array;}
            case 6:case 7:{
                int n=count(in);KeyValuePair[] fields=new KeyValuePair[n];Set<String> names=new HashSet<>();
                for(int i=0;i<n;i++){String name=text(in);if(!names.add(name))throw new HostFailure("protocol","duplicate typed field");fields[i]=KeyValuePair.apply(KeyValue.apply(name),read(in,ctx,depth+1,nodes,inputName),false,false);}
                ObjectValue value=ObjectValue.apply(fields);
                if(kind==7){if(type.isEmpty())throw new HostFailure("protocol","typed object has no class");SchemaProperty property=SchemaProperty.apply(StringValue.apply("class"),(Value)StringValue.apply(type),false,false,false);Schema schema=Schema.apply(new SchemaProperty[]{property},false,false);return ObjectValue.apply(value.evaluate(ctx),value,scala.Option.apply(schema));}
                return value;
            }
            default:throw new HostFailure("protocol","invalid typed value kind");
        }
    }
    private static String className(Value<?> value,EvaluationContext ctx){
        if(value.schema(ctx).isDefined()){var props=value.schema(ctx).get().properties(ctx).iterator();while(props.hasNext()){var property=props.next();if(property.name().evaluate(ctx).equals(Schema.CLASS_PROPERTY_NAME()))return String.valueOf(property.value().evaluate(ctx));}}
        return "";
    }
    private static void write(DataOutputStream out,Value<?> value,EvaluationContext ctx,int depth,int[] nodes,OutputBuffer bytes)throws IOException{
        structure(depth,nodes);String type=className(value,ctx);
        for(int n=0;;n++){if(n>64)throw new HostFailure("structure-limit","typed wrapper depth exceeded");if(value instanceof org.mule.weave.v2.model.values.wrappers.WrapperValue v)value=v.value(ctx);else if(value instanceof org.mule.weave.v2.model.values.wrappers.DelegateValue v)value=v.value(ctx);else break;}
        if(value instanceof QueryRelationship)throw new org.mule.weave.v2.core.exception.WriterExecutionException(value.location(),"application/apex","Invalid type: \"Map_Wrapper\"");
        if(value instanceof NullValue){out.writeByte(0);text(out,type);}
        else if(value instanceof StringValue v){out.writeByte(1);text(out,type);text(out,v.evaluate(ctx));}
        else if(value instanceof CharSequenceValue v){out.writeByte(1);text(out,type);text(out,v.evaluate(ctx).toString());}
        else if(value instanceof NumberValue v){out.writeByte(3);text(out,type);text(out,v.evaluate(ctx).toBigDecimal().bigDecimal().toPlainString());}
        else if(value instanceof BooleanValue v){out.writeByte(4);text(out,type);out.writeByte(Boolean.TRUE.equals(v.evaluate(ctx))?1:0);}
        else if(value instanceof LocalDateValue v){out.writeByte(9);text(out,"Date");text(out,v.evaluate(ctx).toString());}
        else if(value instanceof DateTimeValue v){out.writeByte(10);text(out,"Datetime");text(out,v.evaluate(ctx).toInstant().truncatedTo(java.time.temporal.ChronoUnit.SECONDS).toString());}
        else if(value instanceof LocalTimeValue v){out.writeByte(11);text(out,"Time");text(out,v.evaluate(ctx).toString());}
        else if(value instanceof BinaryValue v){
            if(v.evaluate(ctx).size()>LIMIT)throw new HostFailure("output-limit","typed binary exceeds byte limit");
            byte[] binary=BinaryValue.getBytes(v,false,ctx);
            if(binary.length>LIMIT)throw new HostFailure("output-limit","typed binary exceeds byte limit");
            out.writeByte(12);text(out,"Blob");text(out,Base64.getEncoder().encodeToString(binary));
        }
        else if(value instanceof ArrayValue v){
            out.writeByte(5);text(out,type);int position=bytes.size();out.writeInt(0);var iterator=v.evaluate(ctx).toIterator();int actual=0;while(iterator.hasNext()){if(++actual>NODES)throw new HostFailure("structure-limit","typed list too large");write(out,iterator.next(),ctx,depth+1,nodes,bytes);}bytes.countAt(position,actual);
        }else if(value instanceof ObjectValue v){
            if(type.isEmpty())throw new org.mule.weave.v2.core.exception.WriterExecutionException(value.location(),"application/apex","Need to specify Apex object type with 'as' clause");
            out.writeByte(7);text(out,type);int position=bytes.size();out.writeInt(0);var iterator=v.evaluate(ctx).toIterator(ctx);int actual=0;while(iterator.hasNext()){if(++actual>NODES)throw new HostFailure("structure-limit","typed object too large");var field=iterator.next();text(out,field._1().evaluate(ctx).name());write(out,field._2(),ctx,depth+1,nodes,bytes);}bytes.countAt(position,actual);
        }else throw new HostFailure("unsupported-typed-output","unsupported DataWeave Apex output type: "+value.valueType(ctx));
        if(bytes.size()>LIMIT)throw new HostFailure("output-limit","typed output exceeds byte limit");
    }
}
