# 手动安装的nlohmann/json.hpp路径
include_directories(${CMAKE_CURRENT_SOURCE_DIR}/include)

cmake_minimum_required(VERSION 3.10)
project(pitaya_native LANGUAGES CXX)

set(CMAKE_CXX_STANDARD 17)
set(CMAKE_CXX_STANDARD_REQUIRED ON)
set(CMAKE_CXX_EXTENSIONS OFF)

file(GLOB_RECURSE SRC_FILES 
    "src/*.cpp"
    "include/*.hpp"
)

# 静态库
add_library(pitaya_native STATIC ${SRC_FILES})

target_include_directories(pitaya_native 
    PUBLIC 
    ${CMAKE_CURRENT_SOURCE_DIR}/include
)

# 可执行文件
add_executable(pitaya_main src/main.cpp)
target_link_libraries(pitaya_main pitaya_native)

# 编译选项
if(MSVC)
    target_compile_options(pitaya_native PRIVATE /W4 /WX)
    target_compile_options(pitaya_main PRIVATE /W4 /WX)
else()
    target_compile_options(pitaya_native PRIVATE -Wall -Wextra -Werror)
    target_compile_options(pitaya_main PRIVATE -Wall -Wextra -Werror)
endif()