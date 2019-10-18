package org.eclipse.che.incubator.crd.cherestapis.graalvm;

import java.lang.reflect.Constructor;
import java.lang.reflect.Field;
import java.lang.reflect.GenericArrayType;
import java.lang.reflect.Method;
import java.lang.reflect.ParameterizedType;
import java.lang.reflect.Type;
import java.util.HashSet;
import java.util.Set;

import com.oracle.svm.core.annotate.AutomaticFeature;

import org.eclipse.che.api.workspace.server.devfile.PreferencesDeserializer;
import org.eclipse.che.api.workspace.server.model.impl.devfile.DevfileImpl;
import org.graalvm.nativeimage.hosted.Feature;
import org.graalvm.nativeimage.hosted.RuntimeReflection;

@AutomaticFeature
class RuntimeReflectionRegistrationFeature implements Feature {

  public void beforeAnalysis(BeforeAnalysisAccess access) {
    System.out.println("Eclipse Che compatibility layer for GraalVM native image generation");

    registerFully(DevfileImpl.class);
    registerFully(org.eclipse.che.api.workspace.server.dto.DtoServerImpls.class);
    registerFully(org.eclipse.che.api.core.server.dto.DtoServerImpls.class);
    registerFully(PreferencesDeserializer.class);
  }

  private Set<Class<?>> classesAlreadyRegistered = new HashSet<>();
  private Set<Type> typesAlreadyRegistered = new HashSet<>();

  private void registerFully(Type type) {
    if (typesAlreadyRegistered.contains(type)) {
      return;
    }
    typesAlreadyRegistered.add(type);
    if (type instanceof ParameterizedType) {
      ParameterizedType parameterizedType = (ParameterizedType) type;
      registerFully(parameterizedType.getRawType());
      for (Type paramType : parameterizedType.getActualTypeArguments()) {
        registerFully(paramType);
      }
    } else if (type instanceof GenericArrayType) {
      GenericArrayType genericArrayType = (GenericArrayType) type;
      registerFully(genericArrayType.getGenericComponentType());
    }
    else if (type instanceof Class<?>) {
      registerFully((Class<?>) type);
    }
  }

  private void registerFully(Class<?> clazz) {
    if (classesAlreadyRegistered.contains(clazz)) {
      return;
    }
    if (clazz.getPackage() == null || clazz.getPackage().getName() == null || clazz.getPackage().getName().startsWith("java")) {
      return;
    }
    System.out.println("    =>  Registering class: " + clazz.getName());
    RuntimeReflection.register(clazz);
    classesAlreadyRegistered.add(clazz);
    for (Constructor<?> constructor : clazz.getDeclaredConstructors()) {
      RuntimeReflection.register(constructor);
    }
    for (Method method : clazz.getDeclaredMethods()) {
      RuntimeReflection.register(method);
    }
    for (Field field : clazz.getDeclaredFields()) {
      RuntimeReflection.register(true, field);
      registerFully(field.getGenericType());
    }
    for (Class<?> memberClass : clazz.getDeclaredClasses()) {
      registerFully(memberClass);
    }
    Class<?> superClass = clazz.getSuperclass();
    if (superClass != null) {
      registerFully(superClass);
    }
    Class<?> enclosingClass = clazz.getEnclosingClass();
    if (enclosingClass != null) {
      registerFully(enclosingClass);
    }
  }
}